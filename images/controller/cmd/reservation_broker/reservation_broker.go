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
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
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

type BrokerPod struct {
	Name         string            `json:"name"`
	IP           string            `json:"ip"`
	SessionKey   string            `json:"session_key"`
	UserObjects  []string          `json:"user_objects"`
	SessionStart string            `json:"session_start"`
	UserParams   map[string]string `json:"user_params"`
}

type AppContext struct {
	sync.RWMutex
	Name              string
	AuthHeaderName    string
	UsernameHeader    string
	CookieSecret      string
	PodData           broker.UserPodData
	AvailablePods     []BrokerPod
	ReservedPods      map[string]BrokerPod
	PodWatcherRunning bool
}

type GetPodsSpec struct {
	Items []struct {
		Metadata struct {
			Name              string            `json:"name"`
			Namespace         string            `json:"namespace"`
			CreationTimestamp string            `json:"creationTimestamp"`
			DeletionTimestamp *string           `json:"deletionTimestamp"`
			Annotations       map[string]string `json:"annotations"`
			Labels            map[string]string `json:"labels"`
		} `json:"metadata"`
		Status struct {
			PodIPs []struct {
				IP string `json:"ip"`
			} `json:"podIPs"`
		} `json:"status"`
	} `json:"items"`
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
						Name:              app.Name,
						AuthHeaderName:    authHeaderName,
						UsernameHeader:    usernameHeader,
						CookieSecret:      cookieSecret,
						PodData:           *data,
						AvailablePods:     make([]BrokerPod, 0),
						ReservedPods:      make(map[string]BrokerPod),
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

	// Allow managed pods to query their own session info and themselves down
	sessionFunc := func(w http.ResponseWriter, r *http.Request) {
		srcIP := strings.Split(r.RemoteAddr, ":")[0]
		fwdIP := r.Header.Get("X-Forwarded-For")

		if r.Method == "GET" {
			// Check reserved pods to match requestor IP.
			for _, appCtx := range appContexts {
				for user, pod := range appCtx.ReservedPods {
					if srcIP == pod.IP || fwdIP == pod.IP {
						metadata := broker.ReservationMetadataSpec{
							IP:           pod.IP,
							SessionKey:   pod.SessionKey,
							User:         user,
							SessionStart: pod.SessionStart,
							UserParams:   pod.UserParams,
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						enc := json.NewEncoder(w)
						enc.SetIndent("", "  ")
						enc.Encode(metadata)
						return
					}
				}
			}
			writeResponse(w, http.StatusNotFound, fmt.Sprintf("reservation metadata not found for IP: %s", srcIP))
		} else if r.Method == "DELETE" {
			srcIP := strings.Split(r.RemoteAddr, ":")[0]
			fwdIP := r.Header.Get("X-Forwarded-For")
			// Check reserved pods to match requestor IP.
			for _, appCtx := range appContexts {
				for _, pod := range appCtx.AvailablePods {
					if srcIP == pod.IP || fwdIP == pod.IP {
						statusCode, msg := deletePod(appCtx.Name, pod)
						writeResponse(w, statusCode, msg)
						return
					}
				}
				for user, pod := range appCtx.ReservedPods {
					if srcIP == pod.IP || fwdIP == pod.IP {
						statusCode, msg := deleteApp(appCtx, user)
						writeResponse(w, statusCode, msg)
						return
					}
				}
			}
			writeResponse(w, http.StatusNotFound, fmt.Sprintf("managed pod not found with IP: %s", srcIP))
		} else {
			writeResponse(w, http.StatusBadRequest, fmt.Sprintf("only GET and DELETE methods are supported"))
			return
		}
	}
	server.Urls["session"] = sessionFunc

	// DEPRECATED routes.
	server.Urls["metadata"] = sessionFunc
	server.Urls["shutdown"] = sessionFunc

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
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(status)
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
		// Get user from cookie or header, or check to see if request is coming from a managed pod.
		user := broker.GetUserFromCookieOrAuthHeader(r, cookieName, appCtx.AuthHeaderName)
		var pod BrokerPod
		foundAvailablePod := false
		foundReservedPod := false
		podUser := ""
		if len(user) == 0 {
			// Check to see if request is coming from managed pod.
			srcIP := strings.Split(r.RemoteAddr, ":")[0]
			fwdIP := r.Header.Get("X-Forwarded-For")
			// Check available pods to match requestor IP.
			for _, p := range appCtx.AvailablePods {
				if srcIP == p.IP || fwdIP == p.IP {
					pod = p
					podUser = "none"
					foundAvailablePod = true
					break
				}
			}
			// Check reserved pods to match requestor IP.
			if !foundAvailablePod {
				for u, p := range appCtx.ReservedPods {
					if srcIP == p.IP || fwdIP == p.IP {
						pod = p
						podUser = u
						foundReservedPod = true
						break
					}
				}
			}

			if !foundAvailablePod && !foundReservedPod {
				writeResponse(w, http.StatusUnauthorized, fmt.Sprintf("Failed to get user from cookie or auth header"))
				return
			}

		}

		if foundAvailablePod {
			// Handle request from managed pod
			switch r.Method {
			case "POST":
				writeResponse(w, http.StatusBadRequest, fmt.Sprintf("unsupported request method from source pod without reservation: %s", r.Method))
			case "DELETE":
				status, msg := deletePod(app.Name, pod)
				writeResponse(w, status, msg)
			case "GET":
				msg := "pod has not been reserved"
				writeResponse(w, http.StatusNoContent, msg)
			}
		} else if foundReservedPod {
			// Handle request from reserved pod
			switch r.Method {
			case "POST":
				writeResponse(w, http.StatusBadRequest, fmt.Sprintf("unsupported request method from source pod with reservation: %s", r.Method))
			case "DELETE":
				status, msg := deleteApp(appCtx, user)
				writeResponse(w, status, msg)
			case "GET":
				status, msg := getAppStatus(w, app, appCtx, user, podUser)
				writeResponse(w, status, msg)
			}
		} else {
			// Handle request from user

			// IAP uses a prefix of accounts.google.com:email, remove this to just get the email
			userToks := strings.Split(user, ":")
			user = userToks[len(userToks)-1]

			username := broker.GetUsernameFromHeaderOrDefault(r, appCtx.UsernameHeader, user)

			// Extract any user param values from the request.
			userParams := getUserParams(r, appCtx)

			// Handle each verb
			switch r.Method {
			case "POST":
				status, msg := createApp(app, appCtx, user, username, userParams)
				writeResponse(w, status, msg)
			case "DELETE":
				status, msg := deleteApp(appCtx, user)
				writeResponse(w, status, msg)
			case "GET":
				status, msg := getAppStatus(w, app, appCtx, user, username)
				writeResponse(w, status, msg)
			}
		}
	}
}

/*
Watches a deployment for pods
TODO: convert this to use the K8S watch API.
*/
func watchPods(app broker.AppConfigSpec, appCtx *AppContext) {
	appCtx.PodWatcherRunning = true

	// Get current pod reservations, those not managed by Deployment, reserved for users.
	selector := fmt.Sprintf("%s, app.kubernetes.io/managed-by notin (pod-broker)", app.Deployment.Selector)
	podResp, err := listBrokerPods(app.Name, selector)
	if err != nil {
		log.Printf("failed to list initial reserved pods: %v", err)
	} else {
		appCtx.Lock()
		for _, pod := range podResp.Items {
			if pod.Metadata.DeletionTimestamp != nil {
				// Skip terminating pods
				continue
			}
			podName := pod.Metadata.Name
			podIP := pod.Status.PodIPs[0].IP
			if podUser, ok := pod.Metadata.Annotations["app.broker/user"]; ok {
				sessionKey, ok := pod.Metadata.Annotations["app.broker/session-key"]
				if !ok {
					log.Printf("Warning: missing app.broker/session-key on existing reservation: %s", podName)
				}
				userObjects, ok := pod.Metadata.Annotations["app.broker/last-applied-object-types"]
				if !ok {
					log.Printf("Warning: missing app.broker/last-applied-object-types on existing reservation: %s", podName)
				}
				userParams, ok := pod.Metadata.Annotations["app.broker/user-params"]
				if !ok {
					log.Printf("Warning: missing app.broker/user-params on existing reservation: %s", podName)
				}
				var userParamsDecoded map[string]string
				if err := json.Unmarshal([]byte(userParams), &userParamsDecoded); err != nil {
					log.Printf("Warning: failed to decode JSON user params from app.broker/user-params annotation on pod: %s", podName)
				}

				log.Printf("Found existing reservation: %s: %s", podName, podUser)
				appCtx.ReservedPods[podUser] = BrokerPod{
					Name:        podName,
					IP:          podIP,
					SessionKey:  sessionKey,
					UserObjects: strings.Split(userObjects, ","),
					UserParams:  userParamsDecoded,
				}
			}
		}
		appCtx.Unlock()
	}

	go func() {
		log.Printf("started pod watcher for %s", app.Name)
		for {
			if !appCtx.PodWatcherRunning {
				log.Printf("stopping pod watcher for: %s", app.Name)
				break
			}

			appCtx.Lock()

			// Find available pods, those currently managed by Deployment.
			selector := fmt.Sprintf("%s,app.kubernetes.io/managed-by=pod-broker", app.Deployment.Selector)
			podResp, err := listBrokerPods(app.Name, selector)
			if err != nil {
				log.Printf("failed to list pods for app: %s: %v", app.Name, err)
				time.Sleep(2 * time.Second)
				appCtx.Unlock()
				continue
			}

			// Sort by creation time, descending order.
			sort.Slice(podResp.Items, func(i, j int) bool {
				return podResp.Items[i].Metadata.CreationTimestamp < podResp.Items[j].Metadata.CreationTimestamp
			})

			appCtx.AvailablePods = make([]BrokerPod, 0)
			for _, pod := range podResp.Items {
				if len(pod.Status.PodIPs) == 0 {
					continue
				}
				if pod.Metadata.DeletionTimestamp != nil {
					// Skip terminating pods
					continue
				}
				appCtx.AvailablePods = append(appCtx.AvailablePods, BrokerPod{
					Name: pod.Metadata.Name,
					IP:   pod.Status.PodIPs[0].IP,
				})
			}

			// Write current list of tracked pods for debugging.
			appCtx.WriteCacheFiles()

			appCtx.Unlock()

			time.Sleep(2 * time.Second)
		}
		log.Printf("stopped pod watcher for %s", app.Name)
	}()
}

func updatePodForUser(app broker.AppConfigSpec, user, sessionKey, pod string, objectTypes []string, userParams map[string]string) error {
	// Remove label from the pod that releases it from the K8S Deployment controller.
	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl label pod -n %s %s app.kubernetes.io/managed-by=reservation-broker --overwrite=true 1>&2", app.Name, pod))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%v", stdoutStderr, err)
	}

	// Add broker user annotation
	cmd = exec.Command("sh", "-c", fmt.Sprintf("kubectl annotate pod --overwrite=true -n %s %s 'app.broker/user=%s' 1>&2", app.Name, pod, user))
	stdoutStderr, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%v", stdoutStderr, err)
	}

	// Add session key annotation
	cmd = exec.Command("sh", "-c", fmt.Sprintf("kubectl annotate pod --overwrite=true -n %s %s 'app.broker/session-key=%s' 1>&2", app.Name, pod, sessionKey))
	stdoutStderr, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%v", stdoutStderr, err)
	}

	// Add annotation with found object types
	cmd = exec.Command("sh", "-c", fmt.Sprintf("kubectl annotate pod --overwrite=true -n %s %s 'app.broker/last-applied-object-types=%s' 1>&2", app.Name, pod, strings.Join(objectTypes, ",")))
	stdoutStderr, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%v", stdoutStderr, err)
	}

	// Add annotation for user params.
	encodedUserParams, _ := json.Marshal(&userParams)
	cmd = exec.Command("sh", "-c", fmt.Sprintf("kubectl annotate pod --overwrite=true -n %s %s 'app.broker/user-params=%s' 1>&2", app.Name, pod, string(encodedUserParams)))
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
		cookieValue := broker.MakeCookieValue(user, app.Name, appCtx.CookieSecret)
		appPath := fmt.Sprintf("/%s/", app.Name)
		broker.SetCookie(w, cookieName, cookieValue, appPath, maxCookieAgeSeconds)
	}

	return statusCode, msg
}

/*
Obtain a reservation for the user.
*/
func createApp(app broker.AppConfigSpec, appCtx *AppContext, user, username string, userParams map[string]string) (int, string) {
	statusCode := http.StatusOK
	msg := ""

	// Lock the reservation table so that users get an atomic reservation and they can't reserve multiple pods.
	appCtx.Lock()
	defer appCtx.Unlock()

	if pod, ok := appCtx.ReservedPods[user]; ok {
		msg = fmt.Sprintf("pod for %s: %s", user, pod.Name)
		return statusCode, msg
	}

	if len(appCtx.AvailablePods) == 0 {
		statusCode = http.StatusNotFound
		msg = "No available instances at this time"
		return statusCode, msg
	}

	// Generate session key
	sessionKey := broker.MakeSessionKey()

	// Generate session start timestamp
	ts := fmt.Sprintf("%d", time.Now().Unix())

	// Assign user a pod and remove it from the list
	pod := appCtx.AvailablePods[0]
	appCtx.AvailablePods = appCtx.AvailablePods[1:]
	pod.SessionKey = sessionKey
	pod.SessionStart = ts

	// Build the per-user manifest templates
	destDir, err := buildUserBundle(app, appCtx, user, username, pod)
	if err != nil {
		log.Printf("failed to build user bundle for %s/%s: %v", app.Name, user, err)
		statusCode = http.StatusInternalServerError
		msg = "error creating app"
		return statusCode, msg
	}

	// Determine the unique kinds of objects being applied and add them to a json patch that will add the list to an annotation.
	// This is done because 'kubectl delete all' does not capture things like CRDs or VirtualService so object can get orphaned.
	// See also: https://github.com/kubernetes/kubectl/issues/151
	userObjects, err := broker.GetObjectTypes(destDir)
	if err != nil {
		log.Printf("failed to determine object types in bundle: %v", err)
		statusCode = http.StatusInternalServerError
		msg = "error creating app"
		return statusCode, msg
	}
	pod.UserObjects = userObjects

	// add user params to the pod
	pod.UserParams = userParams

	// Update the pod for the user
	if err := updatePodForUser(app, user, pod.SessionKey, pod.Name, pod.UserObjects, userParams); err != nil {
		log.Printf("failed to update pod for user %s: %s: %v", user, pod.Name, err)
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

	log.Printf("assigned pod %s to user: %s", pod.Name, user)

	// Reserve pod for user in map
	appCtx.ReservedPods[user] = pod

	msg = fmt.Sprintf("assigned pod: %s", pod.Name)
	return statusCode, msg
}

/*
Builds templates for user specific manifest.
*/
func buildUserBundle(app broker.AppConfigSpec, appCtx *AppContext, user, username string, pod BrokerPod) (string, error) {

	data := appCtx.PodData
	data.User = user
	data.Username = username
	data.CookieValue = broker.MakeCookieValue(user, app.Name, appCtx.CookieSecret)
	data.ID = broker.MakePodID(user)
	data.FullName = fmt.Sprintf("%s-%s", app.Name, data.ID)
	data.Timestamp = fmt.Sprintf("%d", time.Now().Unix())
	data.Resources = make([]string, 0)
	data.Patches = make([]string, 0)
	data.JSONPatchesService = make([]string, 0)
	data.JSONPatchesVirtualService = make([]string, 0)
	data.JSONPatchesDeploy = make([]string, 0)

	// Add sessionKey as app param.
	data.AppParams["sessionKey"] = pod.SessionKey

	// Populate app params from app spec.
	for _, param := range app.AppParams {
		data.AppParams[param.Name] = param.Default
	}

	srcDirUser := path.Join(broker.UserBundleSourceBaseDir, app.Name)
	destDirUser := path.Join(broker.BuildSourceBaseDirUser, user, app.Name)
	if err := broker.BuildDeploy(broker.BrokerCommonBuildSourceBaseDirDeploymentUser, srcDirUser, destDirUser, &data); err != nil {
		return "", err
	}

	return destDirUser, nil
}

/*
Release a reservation and delete the pod.
*/
func deleteApp(appCtx *AppContext, user string) (int, string) {
	statusCode := http.StatusOK
	msg := "shutdown"

	// Lock the reservation table
	appCtx.Lock()

	// Remove the reservation from the table.
	defer delete(appCtx.ReservedPods, user)

	defer appCtx.Unlock()

	if bPod, ok := appCtx.ReservedPods[user]; ok {
		podName := bPod.Name
		// Remove instance label from the pod.
		// This is done so that subsequest GET requests don't return the terminating pod.
		cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl label pod -n %s %s app.kubernetes.io/instance- 1>&2", appCtx.Name, podName))
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("warning: failed to remove instance label from pod: %s: %s\n%v", podName, stdoutStderr, err)
		}

		// Delete the pod from K8S
		log.Printf("deleting pod for user %s: %s", user, podName)

		cmd = exec.Command("sh", "-c", fmt.Sprintf("kubectl delete pod -n %s %s --wait=false 1>&2", appCtx.Name, podName))
		stdoutStderr, err = cmd.CombinedOutput()
		if err != nil {
			log.Printf("failed to delete pod for user %s: %s: %s\n%v", user, podName, stdoutStderr, err)
			statusCode = http.StatusInternalServerError
			msg = "error deleting app"
			return statusCode, msg
		}

		// Delete the per-user resources
		if len(bPod.UserObjects) > 0 {
			objectTypes := strings.Join(bPod.UserObjects, ",")
			fullName := fmt.Sprintf("%s-%s", appCtx.Name, broker.MakePodID(user))
			cmdStr := fmt.Sprintf("kubectl delete %s -n %s -l \"app.kubernetes.io/instance=%s, app.broker/deletion-policy notin (abandon)\" --wait=false", objectTypes, appCtx.Name, fullName)
			cmd = exec.Command("sh", "-o", "pipefail", "-c", cmdStr)
			stdoutStderr, err = cmd.CombinedOutput()
			if err != nil {
				log.Printf("error deleting per-user resources for %s: %v\n%s", user, err, stdoutStderr)
				statusCode = http.StatusInternalServerError
				msg = "error deleting app"
				return statusCode, msg
			}
		}
	}
	return statusCode, msg
}

func deletePod(appName string, pod BrokerPod) (int, string) {
	statusCode := http.StatusOK
	msg := "shutdown"
	podName := pod.Name

	// Delete the pod from K8S
	log.Printf("deleting pod %s", podName)

	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl delete pod -n %s %s --wait=false 1>&2", appName, podName))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("failed to delete pod %s: %s\n%v", podName, stdoutStderr, err)
		statusCode = http.StatusInternalServerError
		msg = "error deleting app"
		return statusCode, msg
	}

	return statusCode, msg
}

func listBrokerPods(namespace, selector string) (GetPodsSpec, error) {
	var resp GetPodsSpec

	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get pod -n %s -l \"%s\" -o json 1>&2", namespace, selector))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return resp, fmt.Errorf("failed to list pods: %s, %v", string(stdoutStderr), err)
	}

	var podResp GetPodsSpec
	if err := json.Unmarshal(stdoutStderr, &podResp); err != nil {
		return resp, fmt.Errorf("failed to parse pod spec in initial pod list: %v", err)
	}
	return podResp, nil
}

// WriteCacheFiles is not thread-safe, should be run within the context of a mutex lock.
func (appCtx *AppContext) WriteCacheFiles() {
	availablePodNames := make([]string, 0)
	reservedPodNames := make([]string, 0)

	for _, pod := range appCtx.AvailablePods {
		availablePodNames = append(availablePodNames, pod.Name)
	}

	for _, pod := range appCtx.ReservedPods {
		reservedPodNames = append(reservedPodNames, pod.Name)
	}

	availablePodsCacheFile := path.Join(broker.BundleSourceBaseDir, appCtx.Name, "reservation_pods_available.txt")
	ioutil.WriteFile(availablePodsCacheFile, []byte(strings.Join(availablePodNames, "\n")), 0644)

	reservedPodsCacheFile := path.Join(broker.BundleSourceBaseDir, appCtx.Name, "reservation_pods_reserved.txt")
	ioutil.WriteFile(reservedPodsCacheFile, []byte(strings.Join(reservedPodNames, "\n")), 0644)
}

func getUserParams(r *http.Request, appCtx *AppContext) map[string]string {
	resp := make(map[string]string, 0)

	// Extract query parameters
	// Note that only the first instance of a repeated query param is used.
	queryParams := make(map[string]string, len(r.URL.Query()))
	for k, v := range r.URL.Query() {
		queryParams[k] = v[0]
	}

	// Validate input parameters
	// Only write parameters that were found in the app config and are writable.
	for _, appParam := range appCtx.PodData.AppSpec.UserParams {
		// Return error if param is not found or not writable.
		if v, ok := queryParams[appParam.Name]; ok {
			for _, p := range appCtx.PodData.AppSpec.UserWritableParams {
				if p == appParam.Name {
					// Param is writable, add to return map
					resp[appParam.Name] = v
					break
				}
			}
		}
	}

	return resp
}
