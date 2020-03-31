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
	"net/http"
	"os/exec"
	"regexp"
)

func MakePodID(user string) string {
	h := sha1.New()
	io.WriteString(h, user)
	return fmt.Sprintf("%x", h.Sum(nil))[:10]
}

func GetPodStatus(namespace, selector string) (StatusResponse, error) {
	var resp StatusResponse
	var err error

	type podStatusCondition struct {
		Type   string `json:"type"`
		Status string `json:"status"`
	}

	type podStatusSpec struct {
		PodIP      string               `json:"podIP"`
		Conditions []podStatusCondition `json:"conditions"`
	}

	type podSpec struct {
		Metadata map[string]interface{} `json:"metadata"`
		Status   podStatusSpec          `json:"status"`
	}

	type getPodsSpec struct {
		Items []podSpec `json:"items"`
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

	podStatus := PodStatusResponse{}

	for _, item := range podResp.Items {
		// Status is terminating if metadata.deletionTimestamp is set.
		// https://github.com/kubernetes/kubernetes/issues/22839
		if item.Metadata["deletionTimestamp"] != nil {
			resp.Status = "terminating"
			return resp, err
		}

		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" {
				if cond.Status == "True" {
					resp.PodIPs = append(resp.PodIPs, item.Status.PodIP)
					podStatus.Ready++
				} else {
					podStatus.Waiting++
				}
			}
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

func ValidateImageRepo(repo, tag string, authorizedImagePattern *regexp.Regexp) error {
	// Verifies that the image repo is in the correct format.
	// Verifies pod broker has access to the repo.
	// Verifies that node has access to the repo.

	if !authorizedImagePattern.MatchString(repo) {
		return fmt.Errorf("rejected image repository '%s' per broker config.", repo)
	}

	listResp, err := ListGCRImageTagsInternalMetadataToken(repo)
	if err != nil {
		return fmt.Errorf("failed to check image repository: '%s'", repo)
	}

	if len(listResp.Tags) == 0 {
		return fmt.Errorf("invalid permissions or no tags found for image: '%s", repo)
	}

	return nil
}

func MakeCookieValue(user, cookieSecret string) string {
	// Create cookie value in the form of: user#sha1("user.secret")
	// Note that this value is used in a regex match for virtualservice routing
	// and should be free of regex breaking characters.
	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%s.%s", user, cookieSecret))
	return fmt.Sprintf("%s#%x", user, h.Sum(nil))
}
