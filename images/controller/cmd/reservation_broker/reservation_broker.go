/*
 Copyright 2020 Google Inc. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	broker "selkies.io/controller/pkg"
)

// Cookie max-age in seconds, 5 days.
const maxCookieAgeSeconds = 432000

// Wraps server muxer, dynamic map of handlers, and listen port.
type Server struct {
	Dispatcher *mux.Router
	Urls       map[string]func(w http.ResponseWriter, r *http.Request)
	Port       string
}

type AppContext struct {
	sync.RWMutex
	AuthHeaderName    string
	UsernameHeader    string
	CookieSecret      string
	PodData           broker.UserPodData
	AvailablePods     []string
	ReservedPods      map[string]string
	PodWatcherRunning bool
}

func main() {
	cookieSecret := os.Getenv("COOKIE_SECRET")
	if len(cookieSecret) == 0 {
		// Generate random secret
		log.Printf("no COOKIE_SECRET env var found, generating random secret value")
		h := sha1.New()
		io.WriteString(h, fmt.Sprintf("%d.%d", rand.Intn(10000), int32(time.Now().Unix())))
		cookieSecret = fmt.Sprintf("%x", h.Sum(nil))
	}

	clientID := os.Getenv("OAUTH_CLIENT_ID")
	if len(clientID) == 0 {
		log.Fatalf("missing env, OAUTH_CLIENT_ID")
	}

	// Project ID from instance metadata
	projectID, err := broker.GetProjectID()
	if err != nil {
		log.Fatalf("failed to determine project ID: %v", err)
	}

	// Region from instance metadata
	brokerRegion, err := broker.GetInstanceRegion()
	if err != nil {
		log.Fatalf("failed to determine broker region: %v", err)
	}

	// Values available to templates from environment variables prefixed with POD_BROKER_PARAM_Name=Value
	// Map of Name=Value
	sysParams := broker.GetEnvPrefixedVars("POD_BROKER_PARAM_")

	// Domain from params
	domain, ok := sysParams["Domain"]
	if !ok {
		log.Fatal("Missing POD_BROKER_PARAM_Domain env.")
	}

	// AuthHeader from params
	authHeaderName, ok := sysParams["AuthHeader"]
	if !ok {
		log.Fatal("Missing POD_BROKER_PARAM_AuthHeader env.")
	}

	usernameHeader, _ := sysParams["UsernameHeader"]

	// Period which to scan for apps
	scanPeriod := 5 * time.Second

	// Period which to re-apply manifests
	resyncPeriod := 60 * time.Second

	// Map of cached app manifest checksums
	manifestChecksums := make(map[string]string, 0)

	// Muxed server to handle per-app routes.
	server := &Server{
		Port:       "8082",
		Dispatcher: mux.NewRouter(),
		Urls:       make(map[string]func(w http.ResponseWriter, r *http.Request)),
	}

	// Map of app contexts.
	appContexts := make(map[string]*AppContext, 0)

	// Sync loop for app resources
	go func() {
		lastSync := time.Now()
		for {
			// Discover apps from their config specs located on the filesystem.
			// TODO: look into caching this, large number of apps and http requests can slow down the broker.
			registeredApps, err := broker.NewRegisteredAppManifestFromJSON(broker.RegisteredAppsManifestJSONFile, broker.AppTypeDeployment)
			if err != nil {
				log.Printf("failed to parse registered app manifest: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			for _, app := range registeredApps.Apps {
				if len(app.Deployment.Selector) == 0 {
					log.Printf("error app is missing deployment.selector: %s, skipping", app.Name)
					continue
				}

				// Common variables
				id := broker.MakePodID(app.Name)
				fullName := fmt.Sprintf("%s-%s", app.Name, id)
				namespace := app.Name
				ts := fmt.Sprintf("%d", time.Now().Unix())

				// Verify that the DefaultTier is in the list of NodeTiers and use it.
				var nodeTierSpec broker.NodeTierSpec
				found := false
				for _, tier := range app.NodeTiers {
					if tier.Name == app.DefaultTier {
						nodeTierSpec = tier
						found = true
						break
					}
				}
				if !found {
					log.Printf("Default tier '%s' not found in list of app node tiers", app.DefaultTier)
					continue
				}

				// Build map of app params from app config spec
				appParams := make(map[string]string, 0)
				for _, param := range app.AppParams {
					appParams[param.Name] = param.Default
				}

				// Path to write compiled template output to.
				destDir := path.Join(broker.BuildSourceBaseDir, app.Name)

				// Create template data
				data := &broker.UserPodData{
					Namespace:                 namespace,
					ProjectID:                 projectID,
					ClientID:                  clientID,
					AppSpec:                   app,
					App:                       app.Name,
					ImageRepo:                 app.DefaultRepo,
					ImageTag:                  app.DefaultTag,
					NodeTier:                  nodeTierSpec,
					Domain:                    domain,
					User:                      app.Name,
					Username:                  app.Name,
					ID:                        id,
					FullName:                  fullName,
					ServiceName:               app.ServiceName,
					Resources:                 []string{},
					Patches:                   []string{},
					JSONPatchesService:        []string{},
					JSONPatchesVirtualService: []string{},
					JSONPatchesDeploy:         []string{},
					AppParams:                 appParams,
					SysParams:                 sysParams,
					NetworkPolicyData:         registeredApps.NetworkPolicyData,
					Timestamp:                 ts,
					Region:                    brokerRegion,
				}

				// Build the application bundle.
				srcDirApp := path.Join(broker.BundleSourceBaseDir, app.Name)
				if err := broker.BuildDeploy(broker.BrokerCommonBuildSourceBaseDirDeploymentApp, srcDirApp, destDir, data); err != nil {
					log.Printf("%v", err)
					continue
				}

				var appCtx *AppContext
				if c, ok := appContexts[app.Name]; ok {
					appCtx = c
				} else {
					appCtx = &AppContext{
						AuthHeaderName:    authHeaderName,
						UsernameHeader:    usernameHeader,
						CookieSecret:      cookieSecret,
						PodData:           *data,
						AvailablePods:     make([]string, 0),
						ReservedPods:      make(map[string]string),
						PodWatcherRunning: false,
					}
					appContexts[app.Name] = appCtx
				}

				// Register the app handler
				registerAppHandler(server, app, appCtx)

				// Start the pod watcher
				if !appCtx.PodWatcherRunning {
					watchPods(app, appCtx)
				}

				// Compute and cache checksum to know if we need to re-apply the manifests.
				prevChecksum := manifestChecksums[app.Name]
				if manifestChecksums[app.Name], err = broker.ChecksumDeploy(destDir); err != nil {
					log.Printf("failed to checksum build output directory: %v", err)
					continue
				}
				if prevChecksum != manifestChecksums[app.Name] {
					log.Printf("%s manifest checksum: %s", app.Name, manifestChecksums[app.Name])
				} else {
					now := time.Now()
					if now.Sub(lastSync) >= resyncPeriod {
						lastSync = now
					} else {
						continue
					}
				}

				// Apply manifests them to the cluster.
				log.Printf("deploying manifests for app: %s", destDir)
				cmd := exec.Command("sh", "-o", "pipefail", "-c", fmt.Sprintf("kustomize build %s | kubectl apply -f -", destDir))
				cmd.Dir = destDir
				stdoutStderr, err := cmd.CombinedOutput()
				if err != nil {
					log.Printf("error calling kubectl for %s: %v\n%s", app.Name, err, stdoutStderr)
					continue
				}
			}

			// Prune deleted BrokerAppConfigs, delete namespace and files.
			foundDirs, err := filepath.Glob(path.Join(broker.BuildSourceBaseDir, "*"))
			if err != nil {
				log.Printf("failed to list app directories to prune: %v", err)
			}
			for _, dirName := range foundDirs {
				found := false
				for _, app := range registeredApps.Apps {
					if app.Name == path.Base(dirName) {
						found = true
						break
					}
				}

				if !found {
					appName := path.Base(dirName)
					log.Printf("removing app: %s", appName)

					// Stop the pod watcher
					appContexts[appName].PodWatcherRunning = false

					// Remove app context
					delete(appContexts, appName)

					// Remove app from checksum cache
					delete(manifestChecksums, appName)

					// Delete the app namespace
					cmd := exec.Command("sh", "-o", "pipefail", "-c", fmt.Sprintf("kubectl delete --wait=false ns %s", appName))
					stdoutStderr, err := cmd.CombinedOutput()
					if err != nil {
						log.Printf("error calling kubectl to delete namespace %s: %v\n%s", appName, err, stdoutStderr)
					}

					// Delete the app directory
					os.RemoveAll(dirName)
				}
			}

			time.Sleep(scanPeriod)
		}
	}()

	server.InitDispatch()
	log.Printf("Initializing request routes...\n")

	server.Start()
}

func (s *Server) Start() {
	log.Printf("Starting server on port: %s \n", s.Port)
	http.ListenAndServe(":"+s.Port, s.Dispatcher)
}

func (s *Server) InitDispatch() {
	d := s.Dispatcher
	d.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeResponse(w, http.StatusOK, "OK")
	})

	d.HandleFunc("/{appName}/", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		appName := vars["appName"]

		s.ProxyCall(w, r, appName)
	})
}

func (s *Server) ProxyCall(w http.ResponseWriter, r *http.Request, fName string) {
	if s.Urls[fName] != nil {
		s.Urls[fName](w, r)
	}
}

func writeResponse(w http.ResponseWriter, statusCode int, message string) {
	status := broker.StatusResponse{
		Code:   statusCode,
		Status: message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(status)
}

/*
Registers handler for app.
Handler dispatches requests for HTTP verbs, POST, DELETE, GET.
*/
func registerAppHandler(s *Server, app broker.AppConfigSpec, appCtx *AppContext) {
	// Register app route handler function
	appName := app.Name
	cookieName := fmt.Sprintf("broker_%s", appName)

	s.Urls[app.Name] = func(w http.ResponseWriter, r *http.Request) {
		// Get user from cookie or header
		user := broker.GetUserFromCookieOrAuthHeader(r, cookieName, appCtx.AuthHeaderName)
		if len(user) == 0 {
			writeResponse(w, http.StatusBadRequest, fmt.Sprintf("Failed to get user from cookie or auth header"))
			return
		}
		// IAP uses a prefix of accounts.google.com:email, remove this to just get the email
		userToks := strings.Split(user, ":")
		user = userToks[len(userToks)-1]

		username := broker.GetUsernameFromHeaderOrDefault(r, appCtx.UsernameHeader, user)

		// Handle each verb
		switch r.Method {
		case "POST":
			status, msg := createApp(app, appCtx, user, username)
			writeResponse(w, status, msg)
		case "DELETE":
			status, msg := deleteApp(app, appCtx, user, username)
			writeResponse(w, status, msg)
		case "GET":
			status, msg := getAppStatus(w, app, appCtx, user, username)
			writeResponse(w, status, msg)
		}
	}
}

/*
Watches a deployment for pods
TODO: convert this to use the K8S watch API.
*/
func watchPods(app broker.AppConfigSpec, appCtx *AppContext) {
	appCtx.PodWatcherRunning = true

	type podMetadata struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace"`
		Annotations map[string]string `json:"annotations"`
		Labels      map[string]string `json:"labels"`
	}

	type podSpec struct {
		Metadata podMetadata `json:"metadata"`
	}

	type getPodsSpec struct {
		Items []podSpec `json:"items"`
	}

	// Get current pod reservations, those not managed by Deployment, reserved for users.
	selector := fmt.Sprintf("%s, app.kubernetes.io/managed-by notin (pod-broker)", app.Deployment.Selector)
	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get pod -n %s -l \"%s\" -o json 1>&2", app.Name, selector))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("failed to list initial reserved pods: %s, %v", string(stdoutStderr), err)
	} else {
		var podResp getPodsSpec
		if err := json.Unmarshal(stdoutStderr, &podResp); err != nil {
			log.Printf("failed to parse pod spec in initial pod list: %v", err)
		}

		appCtx.Lock()
		for _, pod := range podResp.Items {
			podName := pod.Metadata.Name
			if podUser, ok := pod.Metadata.Annotations["app.broker/user"]; ok {
				log.Printf("Found existing reservation: %s: %s", podName, podUser)
				appCtx.ReservedPods[podUser] = podName
			}
		}
		appCtx.Unlock()
	}

	go func() {
		log.Printf("started pod watcher for %s", app.Name)
		for {
			if !appCtx.PodWatcherRunning {
				break
			}

			appCtx.Lock()

			// Find available pods, those currently managed by Deployment.
			selector := fmt.Sprintf("%s,app.kubernetes.io/managed-by=pod-broker", app.Deployment.Selector)
			podNames, err := broker.ListPods(app.Name, selector)
			if err != nil {
				log.Printf("failed to list pods for app: %s: %v", app.Name, err)
				time.Sleep(2 * time.Second)
				appCtx.Unlock()
				continue
			}

			// Update the list of available pods
			appCtx.AvailablePods = podNames
			appCtx.Unlock()

			time.Sleep(2 * time.Second)
		}
		log.Printf("stopped pod watcher for %s", app.Name)
	}()
}

func updatePodForUser(app broker.AppConfigSpec, user, pod string) error {
	// Remove label from the pod that releases it from the K8S Deployment controller.
	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl label pod -n %s %s app.kubernetes.io/managed-by=reservation-broker --overwrite=true 1>&2", app.Name, pod))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%v", stdoutStderr, err)
	}

	// Add annotation
	cmd = exec.Command("sh", "-c", fmt.Sprintf("kubectl annotate pod --overwrite=true -n %s %s 'app.broker/user=%s' 1>&2", app.Name, pod, user))
	stdoutStderr, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%v", stdoutStderr, err)
	}

	// Add label for instance ID
	instanceID := fmt.Sprintf("%s-%s", app.Name, broker.MakePodID(user))
	cmd = exec.Command("sh", "-c", fmt.Sprintf("kubectl label pod -n %s %s app.kubernetes.io/instance=%s --overwrite=true 1>&2", app.Name, pod, instanceID))
	stdoutStderr, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%v", stdoutStderr, err)
	}

	return nil
}

/*
Get status of reservation.
*/
func getAppStatus(w http.ResponseWriter, app broker.AppConfigSpec, appCtx *AppContext, user, username string) (int, string) {
	statusCode := http.StatusOK
	msg := ""

	instanceID := fmt.Sprintf("%s-%s", app.Name, broker.MakePodID(user))
	selector := fmt.Sprintf("app.kubernetes.io/instance=%s", instanceID)
	status, err := broker.GetPodStatus(app.Name, selector)
	if err != nil {
		log.Printf("failed to get pod status for selector: %s: %v", selector, err)
		statusCode = http.StatusInternalServerError
		msg = "error fetching status"
		return statusCode, msg
	}

	msg = status.Status

	if status.Status == "waiting" {
		statusCode = http.StatusCreated
	}

	if status.Status == "ready" {
		statusCode = http.StatusOK
		cookieName := fmt.Sprintf("broker_%s", app.Name)
		cookieValue := broker.MakeCookieValue(user, appCtx.CookieSecret)
		appPath := fmt.Sprintf("/%s/", app.Name)
		broker.SetCookie(w, cookieName, cookieValue, appPath, maxCookieAgeSeconds)
	}

	return statusCode, msg
}

/*
Obtain a reservation for the user.
*/
func createApp(app broker.AppConfigSpec, appCtx *AppContext, user, username string) (int, string) {
	statusCode := http.StatusOK
	msg := ""

	// Lock the reservation table so that users get an atomic reservation and they can't reserve multiple pods.
	appCtx.Lock()
	defer appCtx.Unlock()

	if pod, ok := appCtx.ReservedPods[user]; ok {
		msg = fmt.Sprintf("pod for %s: %s", user, pod)
		return statusCode, msg
	}

	if len(appCtx.AvailablePods) == 0 {
		statusCode = http.StatusNotFound
		msg = "No available instances at this time"
		return statusCode, msg
	}

	// Assign user a pod and remove it from the list
	pod := appCtx.AvailablePods[0]
	appCtx.AvailablePods = appCtx.AvailablePods[1:]
	appCtx.ReservedPods[user] = pod

	// Update the pod for the user
	if err := updatePodForUser(app, user, pod); err != nil {
		log.Printf("failed to update pod for user %s: %s: %v", user, pod, err)
		statusCode = http.StatusInternalServerError
		msg = "error creating app"
		return statusCode, msg
	}

	// Build the per-user manifest templates
	destDir, err := buildUserBundle(app, appCtx, user, username, pod)
	if err != nil {
		log.Printf("failed to build user bundle for %s/%s: %v", app.Name, user, err)
		statusCode = http.StatusInternalServerError
		msg = "error creating app"
		return statusCode, msg
	}

	// Apply the per-user manifests
	cmd := exec.Command("sh", "-o", "pipefail", "-c", fmt.Sprintf("kustomize build %s | kubectl apply -f -", destDir))
	cmd.Dir = destDir
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("error applying per-user manifests for %s: %v\n%s", user, err, stdoutStderr)
		statusCode = http.StatusInternalServerError
		msg = "error creating app"
		return statusCode, msg
	}

	log.Printf("assigned pod %s to user: %s", pod, user)

	msg = fmt.Sprintf("assigned pod: %s", pod)
	return statusCode, msg
}

/*
Builds templates for user specific manifest.
*/
func buildUserBundle(app broker.AppConfigSpec, appCtx *AppContext, user, username, pod string) (string, error) {

	data := appCtx.PodData
	data.User = user
	data.Username = username
	data.CookieValue = broker.MakeCookieValue(user, appCtx.CookieSecret)
	data.ID = broker.MakePodID(user)
	data.FullName = fmt.Sprintf("%s-%s", app.Name, data.ID)
	data.Timestamp = fmt.Sprintf("%d", time.Now().Unix())
	data.Resources = make([]string, 0)
	data.Patches = make([]string, 0)
	data.JSONPatchesService = make([]string, 0)
	data.JSONPatchesVirtualService = make([]string, 0)
	data.JSONPatchesDeploy = make([]string, 0)

	srcDirUser := path.Join(broker.UserBundleSourceBaseDir, app.Name)
	destDirUser := path.Join(broker.BuildSourceBaseDirUser, user)
	if err := broker.BuildDeploy(broker.BrokerCommonBuildSourceBaseDirDeploymentUser, srcDirUser, destDirUser, &data); err != nil {
		return "", err
	}

	return destDirUser, nil
}

/*
Release a reservation and delete the pod.
*/
func deleteApp(app broker.AppConfigSpec, appCtx *AppContext, user, username string) (int, string) {
	statusCode := http.StatusOK
	msg := "shutdown"

	// Lock the reservation table
	appCtx.Lock()

	// Remove the reservation from the table.
	defer delete(appCtx.ReservedPods, user)

	defer appCtx.Unlock()

	if pod, ok := appCtx.ReservedPods[user]; ok {
		// Remove instance label from the pod.
		// This is done so that subsequest GET requests don't return the terminating pod.
		cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl label pod -n %s %s app.kubernetes.io/instance- 1>&2", app.Name, pod))
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("warning: failed to remove instance label from pod: %s: %s\n%v", pod, stdoutStderr, err)
		}

		// Delete the pod from K8S
		log.Printf("deleting pod for user %s: %s", user, pod)

		cmd = exec.Command("sh", "-c", fmt.Sprintf("kubectl delete pod -n %s %s --wait=false 1>&2", app.Name, pod))
		stdoutStderr, err = cmd.CombinedOutput()
		if err != nil {
			log.Printf("failed to delete pod for user %s: %s: %s\n%v", user, pod, stdoutStderr, err)
			statusCode = http.StatusInternalServerError
			msg = "error deleting app"
			return statusCode, msg
		}

		// Build the per-user manifests so we know what to delete
		destDir, err := buildUserBundle(app, appCtx, user, username, pod)
		if err != nil {
			log.Printf("failed to build user bundle for %s/%s: %v", app.Name, user, err)
			statusCode = http.StatusInternalServerError
			msg = "error deleting app"
			return statusCode, msg
		}

		// Delete the per-user manifests
		cmd = exec.Command("sh", "-o", "pipefail", "-c", fmt.Sprintf("kustomize build %s | kubectl delete -f -", destDir))
		cmd.Dir = destDir
		stdoutStderr, err = cmd.CombinedOutput()
		if err != nil {
			log.Printf("error deleting per-user manifests for %s: %v\n%s", user, err, stdoutStderr)
			statusCode = http.StatusInternalServerError
			msg = "error deleting app"
			return statusCode, msg
		}
	}
	return statusCode, msg
}
