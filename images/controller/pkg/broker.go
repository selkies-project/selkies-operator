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

package pod_broker

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const sessionKeyCharset = "abcdefghijklmnopqrstuvwxyz"

func MakePodID(user string) string {
	h := sha1.New()
	io.WriteString(h, user)
	return fmt.Sprintf("%x", h.Sum(nil))[:10]
}

func MakeSessionKey() string {
	return fmt.Sprintf("%s-%s-%s",
		StringWithCharset(3, sessionKeyCharset),
		StringWithCharset(4, sessionKeyCharset),
		StringWithCharset(3, sessionKeyCharset))
}

func GetPodStatus(namespace, selector string) (StatusResponse, error) {
	var resp StatusResponse
	var err error

	type getPodsSpec struct {
		Items []struct {
			Metadata struct {
				CreationTimestamp *string           `json:"creationTimestamp"`
				DeletionTimestamp *string           `json:"deletionTimestamp"`
				Annotations       map[string]string `json:"annotations"`
			} `json:"metadata"`
			Spec   map[string]interface{} `json:"spec"`
			Status struct {
				PodIP      string `json:"podIP"`
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
				ContainerStatuses []struct {
					ContainerID string `json:"containerID"`
					Image       string `json:"image"`
					Name        string `json:"name"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get pod -n %s -l %s -o json 1>&2", namespace, selector))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return resp, fmt.Errorf("failed to get pods: %s, %v", string(stdoutStderr), err)
	}

	var podResp getPodsSpec
	if err := json.Unmarshal(stdoutStderr, &podResp); err != nil {
		return resp, fmt.Errorf("failed to parse pod spec: %v", err)
	}

	resp.Code = http.StatusOK
	resp.Nodes = make([]string, 0)
	resp.Containers = make(map[string]string, 0)
	resp.Images = make(map[string]string, 0)
	resp.SessionKeys = make([]string, 0)
	resp.BrokerObjects = make([]string, 0)

	podStatus := PodStatusResponse{}

	for _, item := range podResp.Items {
		// Status is terminating if metadata.deletionTimestamp is set.
		// https://github.com/kubernetes/kubernetes/issues/22839
		if item.Metadata.DeletionTimestamp != nil {
			resp.Status = "terminating"
			return resp, err
		}

		if sessionKey, ok := item.Metadata.Annotations["app.broker/session-key"]; ok {
			resp.SessionKeys = append(resp.SessionKeys, sessionKey)
		}

		if brokerObjects, ok := item.Metadata.Annotations["app.broker/last-applied-object-types"]; ok {
			resp.BrokerObjects = strings.Split(brokerObjects, ",")
		}

		if item.Metadata.CreationTimestamp != nil {
			resp.CreationTimestamp = *item.Metadata.CreationTimestamp
		}

		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" {
				if cond.Status == "True" {
					resp.PodIPs = append(resp.PodIPs, item.Status.PodIP)
					nodeName := item.Spec["nodeName"]
					if nodeName != nil {
						resp.Nodes = append(resp.Nodes, nodeName.(string))
					}
					podStatus.Ready++
				} else {
					podStatus.Waiting++
				}
			} else if cond.Type == "PodScheduled" && cond.Status == "False" {
				podStatus.Waiting++
			}
		}

		for _, containerStatus := range item.Status.ContainerStatuses {
			resp.Containers[containerStatus.Name] = containerStatus.ContainerID
			resp.Images[containerStatus.Name] = containerStatus.Image
		}
	}

	// Status is shutdown if no pods matched selector
	if len(podResp.Items) == 0 {
		resp.Status = "shutdown"
	}

	// Populate status when we have at least 1 ready pod.
	if podStatus.Ready > 0 {
		resp.PodStatus = &podStatus
	}

	// Status is waiting until all pods are ready.
	if podStatus.Waiting > 0 {
		resp.Status = "waiting"
	}

	// Status is ready when no pods are waiting and we have at least 1 ready pod.
	if podStatus.Waiting == 0 && podStatus.Ready > 0 {
		resp.Status = "ready"
	}

	return resp, err
}

func ValidateImageRepo(repo, tag string, authorizedImagePattern *regexp.Regexp) ([]string, error) {
	// Verifies that the image repo is in the correct format.
	// Verifies pod broker has access to the repo.
	// Verifies that node has access to the repo.

	tags := []string{}

	if !authorizedImagePattern.MatchString(repo) {
		return tags, fmt.Errorf("rejected image repository '%s' per broker config.", repo)
	}

	// Get docker config pull secrets
	dockerConfigs := &DockerConfigsSync{}
	if err := dockerConfigs.Update(DefaultBrokerNamespace); err != nil {
		log.Fatalf("failed to fetch docker auth configs: %v", err)
	}

	foundTags, err := dockerConfigs.ListTags(repo)
	if err != nil {
		log.Printf("failed to list image tags for: %s\n%v", repo, err)
		return tags, err
	}

	if len(foundTags) == 0 {
		return tags, fmt.Errorf("invalid permissions or no tags found for image: '%s", repo)
	}

	tags = foundTags

	found := false
	for _, t := range tags {
		if t == tag {
			found = true
			break
		}
	}
	if !found {
		return tags, fmt.Errorf("repo %s does not have tag: %s", repo, tag)
	}

	return tags, nil
}

func MakeCookieValue(user, app, cookieSecret string) string {
	// Create cookie value in the form of: user#sha1("user.app.secret")
	// Note that this value is used in a regex match for virtualservice routing
	// and should be free of regex breaking characters.
	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%s.%s.%s", user, app, cookieSecret))
	return fmt.Sprintf("%s#%x", user, h.Sum(nil))
}

func GetEgressNetworkPolicyData(podBrokerNamespace string) (NetworkPolicyTemplateData, error) {
	resp := NetworkPolicyTemplateData{}

	// Get kube-dns service ClusterIP
	services, err := GetServiceClusterIP("kube-system", "k8s-app=kube-dns")
	if err != nil {
		return resp, err
	}

	for _, svc := range services.Services {
		if svc.ServiceName == "kube-dns" {
			resp.KubeDNSClusterIP = svc.ClusterIP
		}
	}

	return resp, nil
}

func CopyFileToContainer(namespace, selector, container, srcPath, destPath string) error {
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("cannot copy file to container, srcPath not found: %s", srcPath)
	}

	// Fetch pod name from selector query.
	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get pod -n %s -l %s -o 'jsonpath={..metadata.name}' 1>&2", namespace, selector))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get pods: %s, %v", string(stdoutStderr), err)
	}
	podName := string(stdoutStderr)
	if len(podName) == 0 {
		return fmt.Errorf("cloud not find pod with given selector")
	}
	podName = strings.Split(podName, "\n")[0]

	// Copy file to container
	cmd = exec.Command("sh", "-c", fmt.Sprintf("kubectl cp -n %s -c %s %s %s:%s 1>&2", namespace, container, srcPath, podName, destPath))
	stdoutStderr, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy file to pod command: %s, %v", string(stdoutStderr), err)
	}
	return nil
}

func ExecPodCommand(namespace, selector, container, command string) error {
	// Fetch pod name from selector query.
	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get pod -n %s -l %s -o 'jsonpath={..metadata.name}' 1>&2", namespace, selector))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get pods: %s, %v", string(stdoutStderr), err)
	}
	podName := string(stdoutStderr)
	if len(podName) == 0 {
		return fmt.Errorf("cloud not find pod with given selector")
	}

	podName = strings.Split(podName, "\n")[0]

	splitArgs := []string{"kubectl", "-n", namespace, "exec", podName, "-c", container, "--"}
	splitArgs = append(splitArgs, strings.Split(command, " ")...)

	// Execute command in pod container.
	cmd = exec.Command(splitArgs[0], splitArgs[1:]...)
	stdoutStderr, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to exec pod command: %s, %v", string(stdoutStderr), err)
	}
	return nil
}

func GetUserFromCookieOrAuthHeader(r *http.Request, cookieName, authHeaderName string) string {
	res := ""

	if len(cookieName) > 0 {
		cookie, err := r.Cookie(cookieName)
		if err == nil {
			toks := strings.Split(cookie.Value, "#")
			if len(toks) == 2 {
				res = toks[0]
			}
		} else {
			// search for user in query parameters.
			if keys, ok := r.URL.Query()[cookieName]; ok && len(keys[0]) > 0 {
				toks := strings.Split(keys[0], "#")
				if len(toks) == 2 {
					res = toks[0]
				}
			}
		}
	}

	if len(res) == 0 {
		res = r.Header.Get(authHeaderName)
	}

	return res
}

func GetUsernameFromHeaderOrDefault(r *http.Request, usernameHeader, defaultUsername string) string {
	res := r.Header.Get(usernameHeader)

	if len(res) == 0 {
		res = defaultUsername
	}

	return res
}
