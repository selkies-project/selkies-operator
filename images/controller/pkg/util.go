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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	metadata "cloud.google.com/go/compute/metadata"
)

const GCRImageWithTagPattern = `gcr.io.*:.*$`
const GCRImageWithDigestPattern = `gcr.io.*@sha256.*$`

func GetEnvPrefixedVars(prefix string) map[string]string {
	params := map[string]string{}

	pat := regexp.MustCompile(fmt.Sprintf("%s(.*?)=(.*)$", prefix))

	for _, pair := range os.Environ() {
		ma := pat.FindStringSubmatch(pair)
		if len(ma) > 2 {
			params[ma[1]] = ma[2]
		}
	}

	return params
}

func CopyFile(srcPath, destDir string) error {
	from, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer from.Close()
	to, err := os.OpenFile(path.Join(destDir, path.Base(srcPath)), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer to.Close()
	_, err = io.Copy(to, from)
	return err
}

func SetCookie(w http.ResponseWriter, cookieName, cookieValue, appPath string, maxAgeSeconds int) {
	// Set cookie for header based routing.
	cookie := http.Cookie{Name: cookieName, Value: cookieValue, Path: appPath, MaxAge: maxAgeSeconds}
	http.SetCookie(w, &cookie)
}

func GetServiceAccountFromMetadataServer() (string, error) {
	sa := ""
	url := "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/email"

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		return sa, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return sa, err
	}

	return string(body), nil
}

func GetServiceAccountTokenFromMetadataServer(sa string) (string, error) {
	token := ""
	url := fmt.Sprintf("http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/%s/token", sa)
	type saTokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		return sa, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return sa, err
	}

	var tokenResp saTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return token, err
	}

	return tokenResp.AccessToken, nil
}

func ExtractGCRRepoFromImage(image string) string {
	gcrRepo := ""

	if len(regexp.MustCompile(GCRImageWithTagPattern).FindAllString(image, -1)) > 0 {
		// Extract just the repo/image format from the image, excluding any tag at the end.
		gcrRepo = strings.Split(strings.ReplaceAll(image, "gcr.io/", ""), ":")[0]
	} else if len(regexp.MustCompile(GCRImageWithDigestPattern).FindAllString(image, -1)) > 0 {
		// Extract just the repo/image format from the image, excluding the digest at the end.
		gcrRepo = strings.Split(strings.ReplaceAll(image, "gcr.io/", ""), "@")[0]
	}
	return gcrRepo
}

func ListGCRImageTags(image string, authToken string) (ImageListResponse, error) {
	listResp := ImageListResponse{}
	gcrRepo := ExtractGCRRepoFromImage(image)
	if len(gcrRepo) == 0 {
		return listResp, fmt.Errorf("could not determine tag or digest from image: %s", image)
	}

	url := fmt.Sprintf("https://gcr.io/v2/%s/tags/list", gcrRepo)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Selkies_Controller/1.0")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
	resp, err := client.Do(req)
	if err != nil {
		return listResp, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return listResp, err
	}

	if resp.StatusCode != http.StatusOK {
		return listResp, fmt.Errorf("%v", string(body))
	}

	if err := json.Unmarshal(body, &listResp); err != nil {
		return listResp, err
	}

	return listResp, nil
}

func GetGCRDigestFromTag(repo, tag string, authToken string) (string, error) {
	url := fmt.Sprintf("https://gcr.io/v2/%s/manifests/%s", repo, tag)

	client := &http.Client{}
	req, err := http.NewRequest("HEAD", url, nil)
	req.Header.Set("User-Agent", "Selkies_Controller/1.0")
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error fetching HEAD request, status code: %v", resp.StatusCode)
	}

	digest := resp.Header.Get("docker-content-digest")

	return digest, nil
}

func ListGCRImageTagsInternalMetadataToken(image string) (ImageListResponse, error) {
	listResp := ImageListResponse{}

	// Get service account name from metadata server
	sa, err := GetServiceAccountFromMetadataServer()
	if err != nil {
		return listResp, err
	}

	// Get access token from metadata server
	authToken, err := GetServiceAccountTokenFromMetadataServer(sa)
	if err != nil {
		return listResp, err
	}

	return ListGCRImageTags(image, authToken)
}

func GetImagesOnNode() ([]DockerImage, error) {
	resp := make([]DockerImage, 0)

	cmd := exec.Command("sh", "-c", "docker images --digests --format '{{json .}}'")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return resp, fmt.Errorf("failed to get node images, stdoutpipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		return resp, fmt.Errorf("failed to get node images, command start: %v", err)
	}

	d := json.NewDecoder(stdout)
	for {
		var jsonResp DockerImage
		if err := d.Decode(&jsonResp); err == io.EOF {
			break
		} else if err != nil {
			return resp, err
		}
		resp = append(resp, jsonResp)
	}
	if err := cmd.Wait(); err != nil {
		return resp, fmt.Errorf("failed to get node images, command wait: %v", err)
	}

	return resp, nil
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func GetInstanceRegion() (string, error) {
	region := ""
	zone, err := metadata.Zone()
	if err != nil {
		return region, err
	}
	toks := strings.Split(zone, "-")
	region = strings.Join(toks[0:2], "-")

	return region, nil
}

func GetProjectID() (string, error) {
	if projectID := os.Getenv("PROJECT_ID"); len(projectID) > 0 {
		return projectID, nil
	}
	if projectID := os.Getenv("GOOGLE_PROJECT"); len(projectID) > 0 {
		return projectID, nil
	}
	return metadata.ProjectID()
}

func GetServiceClusterIP(namespace, selector string) (ServiceClusterIPList, error) {
	resp := ServiceClusterIPList{
		Services: make([]ServiceClusterIP, 0),
	}

	type serviceSpec struct {
		ClusterIP string `json:"clusterIP"`
	}

	type getServiceSpec struct {
		Metadata map[string]interface{} `json:"metadata"`
		Spec     serviceSpec            `json:"spec"`
	}

	type getServiceList struct {
		Items []getServiceSpec `json:"items"`
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get service -n %s -l %s -o json 1>&2", namespace, selector))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return resp, fmt.Errorf("failed to get node: %s, %v", string(stdoutStderr), err)
	}

	var jsonResp getServiceList
	if err := json.Unmarshal(stdoutStderr, &jsonResp); err != nil {
		return resp, fmt.Errorf("failed to parse service spec: %v", err)
	}

	for _, service := range jsonResp.Items {
		resp.Services = append(resp.Services, ServiceClusterIP{
			ServiceName: service.Metadata["name"].(string),
			ClusterIP:   service.Spec.ClusterIP,
		})
	}

	return resp, nil
}

func GetJobs(namespace, selector string) ([]GetJobSpec, error) {
	resp := make([]GetJobSpec, 0)

	type getJobsList struct {
		Items []GetJobSpec `json:"items"`
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get jobs -n %s -l %s -o json 1>&2", namespace, selector))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return resp, fmt.Errorf("failed to get jobs: %s, %v", string(stdoutStderr), err)
	}

	var jsonResp getJobsList
	if err := json.Unmarshal(stdoutStderr, &jsonResp); err != nil {
		return resp, fmt.Errorf("failed to parse jobs spec: %v", err)
	}

	return jsonResp.Items, nil
}

func ListPods(namespace, selector string) ([]string, error) {
	resp := make([]string, 0)
	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get pod -n %s -l \"%s\" -o name --sort-by=.metadata.creationTimestamp 1>&2", namespace, selector))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return resp, fmt.Errorf("failed to get pods: %s, %v", string(stdoutStderr), err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(stdoutStderr))
	for scanner.Scan() {
		line := scanner.Text()
		podName := strings.Split(line, "/")[1]
		resp = append(resp, podName)
	}

	return resp, nil
}

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
