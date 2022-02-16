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
	"context"
	"encoding/base64"
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
	"regexp"
	"strings"
	"time"

	metadata "cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/pubsub"
	oauth2 "golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	registry_authn "github.com/google/go-containerregistry/pkg/authn"
	registry_name "github.com/google/go-containerregistry/pkg/name"
	remote_registry "github.com/google/go-containerregistry/pkg/v1/remote"
)

const GCRImageWithoutTagPattern = `gcr.io.*$`
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

func GetDockerRepoFromImage(image string) (string, error) {
	resp := ""
	nameRef, err := registry_name.ParseReference(image)
	if err != nil {
		return resp, err
	}
	resp = nameRef.Context().Name()
	return resp, nil
}

// Test each auth config against the list tags operation and return the first working one.
func GetDockerAuthConfigForRepo(image string, authConfigs []registry_authn.AuthConfig) (registry_authn.AuthConfig, error) {
	resp := registry_authn.AuthConfig{}

	// If no auth configs were passed, image might be public.
	if len(authConfigs) == 0 {
		_, err := DockerRemoteRegistryGetDigest(image, []registry_authn.AuthConfig{})
		if err == nil {
			return resp, nil
		}
	}

	for _, authConfig := range authConfigs {
		_, err := DockerRemoteRegistryGetDigest(image, []registry_authn.AuthConfig{authConfig})
		if err == nil {
			resp = authConfig
			return resp, nil
		}
	}
	return resp, fmt.Errorf("failed to find auth config for repo")
}

func GetDefaultSADockerConfig() (DockerConfigJSON, error) {
	dockerConfig := DockerConfigJSON{}

	// Get SA email from metadata server
	sa, err := GetServiceAccountFromMetadataServer()
	if err != nil {
		return dockerConfig, err
	}

	// Get token from default service account.
	token, err := GetServiceAccountTokenFromMetadataServer(sa)
	if err != nil {
		return dockerConfig, err
	}

	// Generate AuthConfig from service account token.
	dockerConfig = DockerConfigJSON{
		Auths: map[string]registry_authn.AuthConfig{
			"https://gcr.io": registry_authn.AuthConfig{
				Username: "oauth2accesstoken",
				Password: token,
			},
		},
	}

	return dockerConfig, nil
}

func DockerRemoteRegistryListTags(image string, authConfigs []registry_authn.AuthConfig) ([]string, error) {
	resp := []string{}
	uaOpt := remote_registry.WithUserAgent("Selkies_Controller/1.0")
	nameRef, err := registry_name.ParseReference(image)
	if err != nil {
		return resp, err
	}
	repoName := nameRef.Context().Name()
	repo, err := registry_name.NewRepository(repoName)
	if err != nil {
		return resp, err
	}
	// If no auth configs were passed, image might be public.
	if len(authConfigs) == 0 {
		resp, err = remote_registry.List(repo, uaOpt)
		if err == nil {
			return resp, nil
		}
	}

	for _, authConfig := range authConfigs {
		authOpt := remote_registry.WithAuth(registry_authn.FromConfig(authConfig))
		resp, err = remote_registry.List(repo, authOpt, uaOpt)
		if err == nil {
			return resp, nil
		}
	}
	return resp, fmt.Errorf("failed to list tags on repo: '%s' with given auth configs", repoName)
}

func DockerRemoteRegistryGetDigest(repoRef string, authConfigs []registry_authn.AuthConfig) (string, error) {
	resp := ""
	nameRef, err := registry_name.ParseReference(repoRef)
	if err != nil {
		return resp, err
	}
	uaOpt := remote_registry.WithUserAgent("Selkies_Controller/1.0")

	// If no auth configs were passed, image might be public.
	if len(authConfigs) == 0 {
		head, err := remote_registry.Head(nameRef, uaOpt)
		if err == nil {
			resp = head.Digest.String()
			return resp, nil
		}
	}

	for _, authConfig := range authConfigs {
		authOpt := remote_registry.WithAuth(registry_authn.FromConfig(authConfig))
		head, err := remote_registry.Head(nameRef, authOpt, uaOpt)
		if err == nil {
			resp = head.Digest.String()
			return resp, nil
		}
	}
	return resp, fmt.Errorf("failed to get digest for: '%s' with given auth configs", repoRef)
}

func GetDockerTagFromImage(image string) (string, error) {
	resp := ""
	tag, err := registry_name.NewTag(image)
	if err != nil {
		return resp, err
	}
	resp = tag.TagStr()
	return resp, nil
}

func GetDockerImageRegistryURI(image string) (string, error) {
	resp := ""
	tag, err := registry_name.NewTag(image)
	if err != nil {
		return resp, err
	}
	resp = tag.RegistryStr()
	return resp, nil
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
	} else if len(regexp.MustCompile(GCRImageWithoutTagPattern).FindAllString(image, -1)) > 0 {
		// Already have repo without tag.
		gcrRepo = strings.ReplaceAll(image, "gcr.io/", "")
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
	req.Header.Set("Accept", "*/*")
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

// Returns a list of all the dockerconfigjson type secrets found in the given namespace.
func GetDockerConfigs(namespace string) ([]DockerConfigJSON, []string, error) {
	resp := make([]DockerConfigJSON, 0)
	secrets := make([]string, 0)

	type getSecretSpec struct {
		Metadata map[string]interface{} `json:"metadata"`
		Data     map[string]interface{} `json:"data"`
	}

	type getSecretList struct {
		Items []getSecretSpec `json:"items"`
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get secret -n %s --field-selector 'type=kubernetes.io/dockerconfigjson' -o json 1>&2", namespace))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return resp, secrets, fmt.Errorf("failed to get node: %s, %v", string(stdoutStderr), err)
	}

	var jsonResp getSecretList
	if err := json.Unmarshal(stdoutStderr, &jsonResp); err != nil {
		return resp, secrets, fmt.Errorf("failed to parse secret list spec: %v", err)
	}

	for _, secret := range jsonResp.Items {
		secretName := secret.Metadata["name"].(string)
		secrets = append(secrets, secretName)
		if v, ok := secret.Data[".dockerconfigjson"]; ok {
			dockerConfigJSONData, err := base64.StdEncoding.DecodeString(v.(string))
			if err != nil {
				return resp, secrets, fmt.Errorf("failed to decode .dockerconfigjson from secret: %s", secretName)
			}

			var dockerConfig DockerConfigJSON
			if err := json.Unmarshal(dockerConfigJSONData, &dockerConfig); err != nil {
				return resp, secrets, fmt.Errorf("failed to parse docker config from secret %s: %v", secretName, err)
			}

			resp = append(resp, dockerConfig)
		}
	}

	return resp, secrets, nil
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

func CleanupDockerImagesOnNode() (string, error) {
	out := ""
	cmd := exec.Command("sh", "-c", "docker images --filter dangling=true -q | xargs -I {} docker rmi {}")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return out, fmt.Errorf("failed to clean node images, stdoutpipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		return out, fmt.Errorf("failed to clean node images, command start: %v", err)
	}
	o, _ := ioutil.ReadAll(stdout)
	out = string(o)
	return out, nil
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

func CopySecret(namespace, name, destDir string) error {
	err := os.MkdirAll(destDir, os.ModePerm)
	if err != nil {
		return err
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get secret -n %s %s -o yaml > %s/resource-%s.yaml", namespace, name, destDir, name))
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy secret: %s, %v", string(stdoutStderr), err)
	}
	return nil
}

func CopyDockerRegistrySecrets(namespace, destDir string) ([]string, error) {
	resp := []string{}
	err := os.MkdirAll(destDir, os.ModePerm)
	if err != nil {
		return resp, err
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get secret -n %s --field-selector 'type=kubernetes.io/dockerconfigjson' -o jsonpath='{range .items[*]}{.metadata.name}{\"\\n\"}{end}' | xargs -I{} sh -c \"kubectl get secret {} -n %s -o yaml > %s/resource-{}.yaml && echo {}\"", namespace, namespace, destDir))
	output, err := cmd.Output()
	if err != nil {
		stdoutStderr, err := cmd.CombinedOutput()
		return resp, fmt.Errorf("failed to copy secret: %s, %v", string(stdoutStderr), err)
	}

	resp = strings.Split(strings.Trim(string(output), " \n"), "\n")
	return resp, nil
}

func GetPubSubSubscription(subName, topicName, project, saEmail string) (*pubsub.Subscription, error) {
	var sub *pubsub.Subscription

	var opts []option.ClientOption
	opts = append(opts, option.WithTokenSource(oauth2.ComputeTokenSource(saEmail)))

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, project, opts...)
	if err != nil {
		return sub, fmt.Errorf("failed to create pubsub client: %v", err)
	}

	// Verify topic exists
	topic := client.Topic(topicName)
	ok, err := topic.Exists(ctx)
	if err != nil {
		return sub, fmt.Errorf("fmt.Errorfailed to check if topic exists: %v", err)
	}
	if !ok {
		return sub, fmt.Errorf("topic does not exist: %s", topicName)
	}

	// Check for existing subscription for this node, create one if needed.
	sub = client.Subscription(subName)
	ok, err = sub.Exists(ctx)
	if err != nil {
		return sub, fmt.Errorf("failed to check if subscription exists: %v", err)
	}
	if !ok {
		log.Printf("creating subscription: %s", subName)
		var err error
		sub, err = client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{Topic: topic})
		if err != nil {
			return sub, fmt.Errorf("failed to create pubsub subscription: %v", err)
		}
	}

	return sub, nil
}

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func K8sTimestampToUnix(k8sTimestamp string) string {
	t, err := time.Parse(time.RFC3339, k8sTimestamp)
	if err != nil {
		log.Printf("WARN: failed to parse timestamp '%s': %v", k8sTimestamp, err)
		return ""
	}
	return fmt.Sprintf("%d", t.Unix())
}
