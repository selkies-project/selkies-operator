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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

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
	req.Header.Add("Metadata-Flavor", "Google")
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
	req.Header.Add("Metadata-Flavor", "Google")
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

func ListGCRImageTags(image string, authToken string) (ImageListResponse, error) {
	listResp := ImageListResponse{}

	// Extract just the repo/image format from the image, excluding any tag at the end.
	gcrRepo := strings.Split(strings.ReplaceAll(image, "gcr.io/", ""), ":")[0]

	url := fmt.Sprintf("https://gcr.io/v2/%s/tags/list", gcrRepo)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", authToken))
	resp, err := client.Do(req)
	if err != nil {
		return listResp, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return listResp, err
	}

	if err := json.Unmarshal(body, &listResp); err != nil {
		return listResp, err
	}

	return listResp, nil
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

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
