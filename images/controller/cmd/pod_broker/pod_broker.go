/*
 Copyright 2019 Google Inc. All rights reserved.

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
	"net/http/httputil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	broker "selkies.io/controller/pkg"
)

// Cookie max-age in seconds, 5 days.
const maxCookieAgeSeconds = 432000

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

	// Values available to templates from environment variables prefixed with POD_BROKER_PARAM_Name=Value
	// Map of Name=Value
	sysParams := broker.GetEnvPrefixedVars("POD_BROKER_PARAM_")

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

	// Title from params
	brokerName, ok := sysParams["Title"]
	if !ok {
		brokerName = "App Launcher"
		log.Printf("using default broker title: %s", brokerName)
	}

	// Theme from params
	brokerTheme, ok := sysParams["Theme"]
	if !ok {
		brokerTheme = "light"
		log.Printf("using default broker theme: %s", brokerTheme)
	}

	// Domain from params
	domain, ok := sysParams["Domain"]
	if !ok {
		log.Fatal("Missing POD_BROKER_PARAM_Domain env.")
	}

	// Logout URL from params
	logoutURL, ok := sysParams["LogoutURL"]
	if !ok {
		logoutURL = fmt.Sprintf("https://%s/_gcp_iap/clear_login_cookie", domain)
	}

	// AuthHeader from params
	authHeaderName, ok := sysParams["AuthHeader"]
	if !ok {
		log.Fatal("Missing POD_BROKER_PARAM_AuthHeader env.")
	}

	usernameHeader, _ := sysParams["UsernameHeader"]

	// Authorized user image repo pattern regexp.
	allowedRepoPatternParam, ok := sysParams["AuthorizedUserRepoPattern"]
	if !ok {
		log.Fatal("Missing POD_BROKER_PARAM_AuthorizedUserRepoPattern env.")
	}
	allowedRepoPattern := regexp.MustCompile(allowedRepoPatternParam)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := sysParams["Debug"]; ok {
			data, _ := httputil.DumpRequest(r, false)
			log.Println(string(data))
		}

		// Discover apps from their config specs located on the filesystem.
		// TODO: look into caching this, large number of apps and http requests can slow down the broker.
		registeredApps, err := broker.NewRegisteredAppManifestFromJSON(broker.RegisteredAppsManifestJSONFile, broker.AppTypeAll)
		if err != nil {
			log.Printf("failed to parse registered app manifest: %v", err)
			writeResponse(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Extract app name from path
		reqApp := strings.Split(r.URL.Path, "/")[1]

		create := false
		shutdown := false
		getStatus := false

		if r.URL.Path == "/" {
			// Get user from header. At this time the per-app cookie has not been set and is not required.
			user := broker.GetUserFromCookieOrAuthHeader(r, "", authHeaderName)
			if len(user) == 0 {
				writeResponse(w, http.StatusBadRequest, fmt.Sprintf("Failed to get user from auth header"))
				return
			}
			// IAP uses a prefix of accounts.google.com:email, remove this to just get the email
			userToks := strings.Split(user, ":")
			user = userToks[len(userToks)-1]

			username := broker.GetUsernameFromHeaderOrDefault(r, usernameHeader, user)

			// Return list of apps
			appList := broker.AppListResponse{
				BrokerName:   brokerName,
				BrokerTheme:  brokerTheme,
				BrokerRegion: brokerRegion,
				Apps:         make([]broker.AppDataResponse, 0),
				User:         user,
				LogoutURL:    logoutURL,
			}

			for _, app := range registeredApps.Apps {
				srcPath := path.Join(broker.BundleSourceBaseDir, app.Name)
				if _, err := os.Stat(srcPath); os.IsNotExist(err) {
					log.Printf("WARN: missing bundle source directory for app: %s", app.Name)
					continue
				}

				if app.UserParams == nil {
					// default user params to empty list.
					app.UserParams = make([]broker.AppConfigParam, 0)
				}

				if len(app.LaunchURL) == 0 {
					// default launch url to prefixed path.
					app.LaunchURL = fmt.Sprintf("/%s/", app.Name)
				}

				// App is editable if user is in the list of editors.
				editable := false
				for _, appEditor := range app.Editors {
					re, err := regexp.Compile(appEditor)
					if err != nil {
						log.Printf("failed to parse app editor as regexp: '%s'.", appEditor)
						continue
					}
					if re.MatchString(user) {
						editable = true
						break
					}
				}

				// Filter app by authorizedUsers if present.
				if len(app.AuthorizedUsers) > 0 {
					found := false
					for _, u := range app.AuthorizedUsers {
						re, err := regexp.Compile(u)
						if err != nil {
							log.Printf("failed to parse authorizedUser as regexp: '%s', skipping app.", u)
							break
						}
						if re.MatchString(user) || re.MatchString(username) {
							found = true
							break
						}
					}
					if !found {
						// Skip this app.
						continue
					}
				}

				appData := broker.AppDataResponse{
					Name:           app.Name,
					Type:           app.Type,
					DisplayName:    app.DisplayName,
					Description:    app.Description,
					Icon:           app.Icon,
					LaunchURL:      app.LaunchURL,
					DefaultRepo:    app.DefaultRepo,
					DefaultTag:     app.DefaultTag,
					Params:         app.UserParams,
					DefaultTier:    app.DefaultTier,
					NodeTiers:      app.NodeTierNames(),
					Editable:       editable,
					DisableOptions: app.DisableOptions,
				}
				appList.Apps = append(appList.Apps, appData)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(appList)
			return
		}

		// Get app spec from parsed apps.
		app, ok := registeredApps.Apps[reqApp]
		if !ok {
			log.Printf("app not found: %s", reqApp)
			writeResponse(w, http.StatusNotFound, "app not found")
			return
		}

		if app.Type != broker.AppTypeStatefulSet {
			w.Header().Set("Location", fmt.Sprintf("/reservation-broker/%s/", app.Name))
			writeResponse(w, http.StatusFound, "app is reservation type")
			return
		}

		appName := app.Name

		cookieName := fmt.Sprintf("broker_%s", appName)

		switch r.Method {
		case "POST":
			create = true
		case "DELETE":
			shutdown = true
		case "GET":
			getStatus = true
		}

		// Get user from cookie or header
		user := broker.GetUserFromCookieOrAuthHeader(r, cookieName, authHeaderName)
		if len(user) == 0 {
			writeResponse(w, http.StatusBadRequest, fmt.Sprintf("Failed to get user from cookie or auth header"))
			return
		}
		// IAP uses a prefix of accounts.google.com:email, remove this to just get the email
		userToks := strings.Split(user, ":")
		user = userToks[len(userToks)-1]

		username := broker.GetUsernameFromHeaderOrDefault(r, usernameHeader, user)

		// App is editable if user is in the list of editors.
		editable := false
		for _, appEditor := range app.Editors {
			re, err := regexp.Compile(appEditor)
			if err != nil {
				log.Printf("failed to parse app editor as regexp: '%s'.", appEditor)
				continue
			}
			if re.MatchString(user) || re.MatchString(username) {
				editable = true
				break
			}
		}

		// Check per-app user authorization if present.
		if len(app.AuthorizedUsers) > 0 {
			found := false
			for _, u := range app.AuthorizedUsers {
				re, err := regexp.Compile(u)
				if err != nil {
					log.Printf("failed to parse authorizedUser as regexp: '%s', skipping app.", u)
					break
				}
				if re.MatchString(user) || re.MatchString(username) {
					found = true
					break
				}
			}
			if !found {
				writeResponse(w, http.StatusUnauthorized, fmt.Sprintf("user is not authorized"))
				return
			}
		}

		// Compute pod ID from user and app, must conform to DNS-1035.
		id := broker.MakePodID(user)

		namespace := fmt.Sprintf("user-%s", id)

		fullName := fmt.Sprintf("%s-%s", appName, id)

		destDir := path.Join(broker.BuildSourceBaseDir, user, appName)

		cookieValue := broker.MakeCookieValue(user, cookieSecret)

		userConfigFile := path.Join(broker.AppUserConfigBaseDir, appName, user, broker.AppUserConfigJSONFile)

		ts := fmt.Sprintf("%d", time.Now().Unix())

		// Fetch user config, only use user options if spec.disableOptions is false.
		userConfig, err := broker.GetAppUserConfig(userConfigFile)
		if app.DisableOptions || err != nil {
			// config does not exist yet, generate default.
			defaultAppParams := make(map[string]string, 0)
			for _, param := range app.UserParams {
				defaultAppParams[param.Name] = param.Default
			}

			// Create new user config field with default spec
			userConfig = broker.NewAppUserConfig(fullName, namespace, broker.AppUserConfigSpec{
				AppName:   appName,
				User:      user,
				ImageRepo: app.DefaultRepo,
				ImageTag:  app.DefaultTag,
				Tags:      []string{app.DefaultTag},
				NodeTier:  app.DefaultTier,
				Params:    defaultAppParams,
			})
		}

		userNSData := &broker.UserPodData{
			Namespace: namespace,
			ProjectID: projectID,
			AppSpec:   app,
			User:      user,
			Timestamp: ts,
		}
		srcDirUser := path.Join(broker.UserBundleSourceBaseDir, appName)
		destDirUser := path.Join(broker.BuildSourceBaseDirNS, user)

		// Handle requests for per-app session info requests
		if regexp.MustCompile(fmt.Sprintf(".*%s/session/?$", appName)).MatchString(r.URL.Path) {
			// Fetch pod status
			status, err := broker.GetPodStatus(namespace, fmt.Sprintf("app.kubernetes.io/instance=%s,app=%s", fullName, app.ServiceName))
			if err != nil {
				log.Printf("failed to get pod status: %v", err)
				writeResponse(w, http.StatusInternalServerError, "internal server error")
				return
			}

			ip := ""
			sessionKey := ""
			if len(status.PodIPs) > 0 {
				ip = status.PodIPs[0]
			}
			if len(status.SessionKeys) > 0 {
				sessionKey = status.SessionKeys[0]
			}
			metadata := broker.ReservationMetadataSpec{
				IP:           ip,
				SessionKey:   sessionKey,
				User:         user,
				SessionStart: broker.K8sTimestampToUnix(status.CreationTimestamp),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(metadata)
			return
		}

		// Handle requests for per-app user configs
		if regexp.MustCompile(fmt.Sprintf(".*%s/config/?$", appName)).MatchString(r.URL.Path) {
			if getStatus {
				statusCode := http.StatusOK
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(statusCode)
				json.NewEncoder(w).Encode(userConfig.Spec)
				return

			} else if create {
				// Read JSON body
				if r.Header.Get("content-type") != "application/json" {
					writeResponse(w, http.StatusBadRequest, "invalid content-type")
					return
				}

				// Verify config is allowed to be set per broker app config.
				if app.DisableOptions {
					writeResponse(w, http.StatusBadRequest, "user config cannot be modified at this time")
					return
				}

				var inputConfigSpec broker.AppUserConfigSpec
				err := json.NewDecoder(r.Body).Decode(&inputConfigSpec)
				if err != nil {
					writeResponse(w, http.StatusBadRequest, "invalid app user config")
					return
				}

				// Overwrite immutable fields.
				inputConfigSpec.AppName = appName
				inputConfigSpec.User = user
				inputConfigSpec.Tags = userConfig.Spec.Tags

				if len(inputConfigSpec.ImageRepo) == 0 {
					writeResponse(w, http.StatusBadRequest, "missing config field: imageRepo")
					return
				}

				if len(inputConfigSpec.ImageTag) == 0 {
					writeResponse(w, http.StatusBadRequest, "missing config field: imageTag")
					return
				}

				if len(inputConfigSpec.NodeTier) == 0 {
					writeResponse(w, http.StatusBadRequest, "missing config field: nodeTier")
					return
				}
				foundTier := false
				for _, tier := range app.NodeTiers {
					if inputConfigSpec.NodeTier == tier.Name {
						foundTier = true
					}
				}
				if !foundTier {
					writeResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid node tier: %s", inputConfigSpec.NodeTier))
					return
				}

				// Verifiy image repo and tag exists if it was changed.
				if inputConfigSpec.ImageRepo != userConfig.Spec.ImageRepo || inputConfigSpec.ImageTag != userConfig.Spec.ImageTag {
					log.Printf("validating user image repo: %s:%s", inputConfigSpec.ImageRepo, inputConfigSpec.ImageTag)
					imageTags, err := broker.ValidateImageRepo(inputConfigSpec.ImageRepo, inputConfigSpec.ImageTag, allowedRepoPattern)
					if err != nil {
						log.Printf("user %s config image validation failed: %v", user, err)
						writeResponse(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
						return
					}
					inputConfigSpec.Tags = imageTags
				}

				// Verify parameters are valid for this app.
				for paramName := range inputConfigSpec.Params {
					found := false
					for _, supportedParamName := range app.UserParams {
						if paramName == supportedParamName.Name {
							found = true
							break
						}
					}
					if !found {
						msg := fmt.Sprintf("user %s config image validation failed: invalid parameter '%s", user, paramName)
						log.Printf(msg)
						writeResponse(w, http.StatusBadRequest, msg)
						return
					}
				}

				// Set user config spec to validated input spec.
				userConfig.Spec = inputConfigSpec

				// Save config to local file.
				if err := userConfig.WriteJSON(userConfigFile); err != nil {
					log.Printf("failed to save copy of user config: %v", err)
					writeResponse(w, http.StatusInternalServerError, "internal server error")
					return
				}

				// Build user namespace template.
				if err := broker.BuildDeploy(broker.BrokerCommonBuildSourceBaseDirStatefulSetUser, srcDirUser, destDirUser, userNSData); err != nil {
					log.Printf("%v", err)
					writeResponse(w, http.StatusInternalServerError, "internal server error")
					return
				}

				// Apply config to cluster
				cmd := exec.Command("sh", "-o", "pipefail", "-c", fmt.Sprintf("kustomize build %s | kubectl apply -f - && kubectl apply -f %s", destDirUser, userConfigFile))
				cmd.Dir = path.Dir(destDir)
				stdoutStderr, err := cmd.CombinedOutput()
				if err != nil {
					log.Printf("error calling kubectl to apply user config for %s: %v\n%s", user, err, stdoutStderr)
					writeResponse(w, http.StatusInternalServerError, "internal server error")
					return
				}

				writeResponse(w, http.StatusOK, "user config updated")

			} else {
				writeResponse(w, http.StatusBadRequest, "only POST method is supported.")
				return
			}
			return
		}

		// Extract query parameters
		// Note that only the first instance of a repeated query param is used.
		queryParams := make(map[string]string, len(r.URL.Query()))
		for k, v := range r.URL.Query() {
			queryParams[k] = v[0]
		}

		// Map named tier to NodeTierSpec for pod template.
		// Spec contains node affinity labels and resources used in pod templates.
		var nodeTierSpec broker.NodeTierSpec
		found := false
		for _, tier := range app.NodeTiers {
			if tier.Name == userConfig.Spec.NodeTier {
				nodeTierSpec = tier
				found = true
				break
			}
		}
		if !found {
			log.Printf("failed to map config tier '%s' to NodeTierSpec", userConfig.Spec.NodeTier)
			writeResponse(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Build map of app params from app config spec
		appParams := make(map[string]string, 0)
		for _, param := range app.AppParams {
			appParams[param.Name] = param.Default
		}

		// Generate session key. Add to AppParams
		if _, ok := appParams["sessionKey"]; !ok {
			appParams["sessionKey"] = broker.MakeSessionKey()
		}

		data := &broker.UserPodData{
			Namespace:                 namespace,
			ProjectID:                 projectID,
			ClientID:                  clientID,
			AppSpec:                   app,
			AppUserConfig:             userConfig.Spec,
			App:                       appName,
			ImageRepo:                 userConfig.Spec.ImageRepo,
			ImageTag:                  userConfig.Spec.ImageTag,
			NodeTier:                  nodeTierSpec,
			Domain:                    domain,
			User:                      user,
			Username:                  username,
			CookieValue:               cookieValue,
			ID:                        id,
			FullName:                  fullName,
			ServiceName:               app.ServiceName,
			Resources:                 []string{},
			Patches:                   []string{},
			JSONPatchesService:        []string{},
			JSONPatchesVirtualService: []string{},
			JSONPatchesDeploy:         []string{},
			UserParams:                userConfig.Spec.Params,
			AppParams:                 appParams,
			SysParams:                 sysParams,
			NetworkPolicyData:         registeredApps.NetworkPolicyData,
			Timestamp:                 ts,
			Region:                    brokerRegion,
			Editable:                  editable,
		}

		appPath := fmt.Sprintf("/%s/", appName)

		// Build user application bundle.
		srcDirApp := path.Join(broker.BundleSourceBaseDir, app.Name)
		if err := broker.BuildDeploy(broker.BrokerCommonBuildSourceBaseDirStatefulSetApp, srcDirApp, destDir, data); err != nil {
			log.Printf("%v", err)
			writeResponse(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Build patch that adds list of applied objects to the statefulset.
		if err := broker.GenerateObjectTypePatch(destDir); err != nil {
			log.Printf("%v", err)
			writeResponse(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Build user namespace template.
		if err := broker.BuildDeploy(broker.BrokerCommonBuildSourceBaseDirStatefulSetUser, srcDirUser, destDirUser, userNSData); err != nil {
			log.Printf("%v", err)
			writeResponse(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if shutdown {
			if _, err := os.Stat(destDir); os.IsNotExist(err) {
				writeResponse(w, http.StatusBadRequest, "shutdown")
				return
			}

			for i, hook := range app.ShutdownHooks {
				selector := strings.Join([]string{"app.kubernetes.io/instance=" + fullName, hook.Selector}, ",")
				log.Printf("executing shutdown hook %d/%d for %s, selector=%s, container=%s, command=%s", i+1, len(app.ShutdownHooks), fullName, selector, hook.Container, hook.Command)
				if err := broker.ExecPodCommand(namespace, selector, hook.Container, hook.Command); err != nil {
					log.Printf("error calling shutdown hook: %v", err)
				}
			}

			// Fetch pod status to retrieve the list of object types.
			status, err := broker.GetPodStatus(namespace, fmt.Sprintf("app.kubernetes.io/instance=%s,app=%s", fullName, app.ServiceName))
			if err != nil {
				log.Printf("failed to get pod status: %v", err)
				writeResponse(w, http.StatusInternalServerError, "internal server error")
				return
			}

			if len(status.BrokerObjects) > 0 {
				log.Printf("shutting down %s pod for user: %s", appName, user)
				objectTypes := strings.Join(status.BrokerObjects, ",")
				cmd := exec.Command("sh", "-o", "pipefail", "-c", fmt.Sprintf("kubectl delete %s -n %s -l \"app.kubernetes.io/instance=%s, app.broker/deletion-policy notin (abandon)\" --wait=false", objectTypes, namespace, fullName))
				cmd.Dir = destDir
				stdoutStderr, err := cmd.CombinedOutput()
				if err != nil {
					log.Printf("error calling kubectl for %s: %v\n%s", user, err, stdoutStderr)
					writeResponse(w, http.StatusInternalServerError, "internal server error")
					return
				}
			}

			// Delete the cookie by setting max-age to -1
			broker.SetCookie(w, cookieName, cookieValue, appPath, -1)

			writeResponse(w, http.StatusAccepted, "terminating")
			return
		}

		if getStatus {
			// Get pod status based on conditions.
			status, err := broker.GetPodStatus(namespace, fmt.Sprintf("app.kubernetes.io/instance=%s,app=%s", fullName, app.ServiceName))
			if err != nil {
				log.Printf("failed to get pod ips: %v", err)
				writeResponse(w, http.StatusInternalServerError, "internal server error")
				return
			}

			statusCode := http.StatusOK

			if status.Status == "waiting" {
				statusCode = http.StatusCreated
			}

			if status.Status == "ready" {
				broker.SetCookie(w, cookieName, cookieValue, appPath, maxCookieAgeSeconds)
			}

			if redirectURL, ok := queryParams["r"]; ok {
				// Add header to redirect.
				w.Header().Set("Location", redirectURL)
				statusCode = http.StatusTemporaryRedirect
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(status)

			return
		}

		if create {
			log.Printf("creating pod for user: %s: %s", user, fullName)
			cmd := exec.Command("sh", "-o", "pipefail", "-c", fmt.Sprintf("kustomize build %s | kubectl apply -f - && kustomize build %s | kubectl apply -f -", destDirUser, destDir))
			cmd.Dir = destDir
			stdoutStderr, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("error calling kubectl for %s: %v\n%s", user, err, stdoutStderr)
				writeResponse(w, http.StatusInternalServerError, "internal server error")
				return
			}

			broker.SetCookie(w, cookieName, cookieValue, appPath, maxCookieAgeSeconds)

			writeResponse(w, http.StatusAccepted, "created")
			log.Printf("pod created for user: %s: %s", user, fullName)
		}
	})

	log.Println("Listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
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

func writeResponseWithIPs(w http.ResponseWriter, statusCode int, message string, ips []string) {
	status := broker.StatusResponse{
		Status: message,
		PodIPs: ips,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(status)
}
