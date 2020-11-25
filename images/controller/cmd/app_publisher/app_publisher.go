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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	broker "selkies.io/controller/pkg"
)

type newAppData struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Entrypoint  string `json:"entrypoint"`
}

type appPublishJobTemplateData struct {
	JobName     string
	Namespace   string
	AppName     string
	User        string
	NodeName    string
	ContainerID string
	ImageTag    string
	ProjectID   string
	NewApp      newAppData
}

func main() {
	log.Printf("Starting broker app publisher service")

	// Set from downward API.
	namespace := os.Getenv("NAMESPACE")
	if len(namespace) == 0 {
		log.Fatal("Missing NAMESPACE env.")
	}

	templatePath := os.Getenv("TEMPLATE_PATH")
	if len(templatePath) == 0 {
		templatePath = "/run/app-publisher/template/app-publish-job.yaml.tmpl"
	}

	// Values available to templates from environment variables prefixed with POD_BROKER_PARAM_Name=Value
	// Map of Name=Value
	sysParams := broker.GetEnvPrefixedVars("POD_BROKER_PARAM_")

	// AuthHeader from params
	authHeaderName, ok := sysParams["AuthHeader"]
	if !ok {
		log.Fatal("Missing POD_BROKER_PARAM_AuthHeader env.")
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := sysParams["Debug"]; ok {
			data, _ := httputil.DumpRequest(r, false)
			log.Println(string(data))
		}

		// Discover apps from their config specs located on the filesystem.
		// TODO: look into caching this, large number of apps and http requests can slow down the broker.
		registeredApps, err := broker.NewRegisteredAppManifestFromJSON(broker.RegisteredAppsManifestJSONFile, broker.AppTypeStatefulSet)
		if err != nil {
			log.Printf("failed to parse registered app manifest: %v", err)
			writeResponse(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// Extract app name from path
		reqApp := strings.Split(r.URL.Path, "/")[1]

		// Get app spec from parsed apps.
		app, ok := registeredApps.Apps[reqApp]
		if !ok {
			log.Printf("app not found: %s", reqApp)
			writeResponse(w, http.StatusNotFound, fmt.Sprintf("app not found: %s", reqApp))
			return
		}

		appName := app.Name

		cookieName := fmt.Sprintf("broker_%s", appName)

		// Get user from cookie or header
		user := broker.GetUserFromCookieOrAuthHeader(r, cookieName, authHeaderName)
		if len(user) == 0 {
			writeResponse(w, http.StatusBadRequest, fmt.Sprintf("Failed to get user from cookie or auth header"))
			return
		}
		// IAP uses a prefix of accounts.google.com:email, remove this to just get the email
		userToks := strings.Split(user, ":")
		user = userToks[len(userToks)-1]

		// App is editable if user is in the list of editors.
		editable := false
		for _, appEditor := range app.Editors {
			if appEditor == user {
				editable = true
				break
			}
		}

		// Return with error if user is unauthorized
		if !editable {
			writeResponse(w, http.StatusUnauthorized, "user is not authorized to publish")
			return
		}

		getRequest := false
		postRequest := false
		deleteRequest := false

		switch r.Method {
		case "POST":
			postRequest = true
		case "DELETE":
			deleteRequest = true
		case "GET":
			getRequest = true
		}

		jobName := fmt.Sprintf("app-publish-%s", appName)

		currJobs, err := broker.GetJobs(namespace, fmt.Sprintf("app=%s", jobName))
		if err != nil {
			writeResponse(w, http.StatusInternalServerError, "failed to query image publish jobs")
			return
		}

		// GET request checks status of current publish jobs for this app.
		// Responses:
		//   StatusOK (200): Idle, no job currently running.
		//   StatusCreated (201): Job is currently running.
		if getRequest {
			if len(currJobs) == 0 {
				writeResponse(w, http.StatusOK, "no active jobs.")
				return
			}

			writeResponse(w, http.StatusCreated, "image publish job is running")
			return
		}

		// POST request creates new job to publish app from existing container.
		if postRequest {
			newApp, err := parseNewAppData(r, app)
			if err != nil {
				writeResponse(w, http.StatusBadRequest, fmt.Sprintf("failed to parse new app data: %v", err))
				return
			}

			// Error if job is already running.
			if len(currJobs) > 0 {
				writeResponse(w, http.StatusTooManyRequests, "app publish job is already running")
				return
			}

			id := broker.MakePodID(user)

			userNamespace := fmt.Sprintf("user-%s", id)

			fullName := fmt.Sprintf("%s-%s", appName, id)

			// Get current status of app pod.
			podStatus, err := broker.GetPodStatus(userNamespace, fmt.Sprintf("app.kubernetes.io/instance=%s,app=%s", fullName, app.ServiceName))
			if err != nil {
				log.Printf("failed to get pod ips: %v", err)
				writeResponse(w, http.StatusInternalServerError, "internal server error")
				return
			}

			if podStatus.Status != "ready" {
				writeResponse(w, http.StatusBadRequest, "pod is not ready")
				return
			}

			if len(podStatus.Nodes) == 0 {
				writeResponse(w, http.StatusNotFound, fmt.Sprintf("no pods found matching: %s", fullName))
				return
			}

			if len(podStatus.Nodes) != 1 {
				log.Printf("Found more than one node for pod instance %s", fullName)
				writeResponse(w, http.StatusBadRequest, "failed to locate pod on single node to publish from")
				return
			}

			nodeName := podStatus.Nodes[0]
			containerID := strings.ReplaceAll(podStatus.Containers["desktop"], "docker://", "")
			image := fmt.Sprintf("gcr.io/%s/vdi-%s:latest", sysParams["ProjectID"], newApp.Name)

			if err := makeAppPublishJob(namespace, jobName, appName, image, nodeName, containerID, user, sysParams["ProjectID"], templatePath, newApp); err != nil {
				log.Printf("failed to create job: %v", err)
				writeResponse(w, http.StatusInternalServerError, "failed to create job")
				return
			}

			writeResponse(w, http.StatusCreated, fmt.Sprintf("Created app publish job: %s", jobName))

			return
		}

		// DELETE request will delete the job.
		if deleteRequest {
			writeResponse(w, http.StatusBadRequest, "NTI")
			return
		}
	})

	log.Println("Listening on port 8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
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

func parseNewAppData(r *http.Request, oldApp broker.AppConfigSpec) (newAppData, error) {
	resp := newAppData{}

	// Read JSON body
	if r.Header.Get("content-type") != "application/json" {
		return resp, fmt.Errorf("invalid content-type, expected: application/json")
	}

	err := json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		return resp, err
	}

	if len(resp.Name) == 0 {
		return resp, fmt.Errorf("missing 'name'")
	}

	if len(resp.DisplayName) == 0 {
		// Default to match name
		resp.DisplayName = resp.Name
	}

	if len(resp.Description) == 0 {
		// Default to old description.
		resp.Description = oldApp.Description
	}

	if len(resp.Icon) == 0 {
		// Default to old icon
		resp.Icon = oldApp.Icon
	}

	return resp, nil
}

func makeAppPublishJob(namespace, jobName, appName, image, nodeName, containerID, user, projectID, templatePath string, newApp newAppData) error {
	log.Printf("creating app publish job: %s, %s, %s", jobName, image, nodeName)

	data := appPublishJobTemplateData{
		JobName:     jobName,
		Namespace:   namespace,
		AppName:     appName,
		User:        user,
		NodeName:    nodeName,
		ContainerID: containerID,
		ImageTag:    image,
		ProjectID:   projectID,
		NewApp:      newApp,
	}

	destDir := path.Join("/run/app-publisher", jobName)
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to make destDir %s: %v", destDir, err)
	}

	base := path.Base(templatePath)
	t, err := template.New(base).Funcs(sprig.TxtFuncMap()).ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to initialize template: %v", err)
	}
	dest, _ := os.Create(strings.ReplaceAll(path.Join(destDir, base), ".tmpl", ""))
	if err != nil {
		return fmt.Errorf("failed to create dest template file: %v", err)
	}
	if err = t.Execute(dest, &data); err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	// Apply the job to the cluster
	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl apply -f %s 1>&2", destDir))
	cmd.Dir = path.Dir(destDir)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error calling kubectl to apply job: %v\n%s", err, string(stdoutStderr))
	}

	return nil
}
