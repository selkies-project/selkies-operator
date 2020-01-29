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
	"log"
	"os"
	"path"
	"time"

	broker "gcp.solutions/anthos-app-broker/pkg"
)

func main() {

	log.Printf("Starting image tag discovery service")

	// Set from downward API.
	namespace := os.Getenv("NAMESPACE")
	if len(namespace) == 0 {
		log.Fatal("Missing NAMESPACE env.")
	}

	for {
		// Fetch all user app configs
		userConfigs, err := broker.FetchAppUserConfigs(namespace)
		if err != nil {
			log.Fatalf("failed to fetch user app configs: %v", err)
		}

		// Get service account name from metadata server
		sa, err := broker.GetServiceAccountFromMetadataServer()
		if err != nil {
			log.Fatalf("failed to get service account name from metadata server: %v", err)
		}

		// Get access token from metadata server
		token, err := broker.GetServiceAccountTokenFromMetadataServer(sa)
		if err != nil {
			log.Fatalf("failed to get token from metadata server: %v", err)
		}

		// Discover image tags in parallel for all app specs.
		for i := range userConfigs {

			destDir := path.Join(broker.AppUserConfigBaseDir, userConfigs[i].Spec.AppName, userConfigs[i].Spec.User)
			err = os.MkdirAll(destDir, os.ModePerm)
			if err != nil {
				log.Fatalf("failed to create directory: %v", err)
			}

			// Fetch in go routine.
			go getImageTags(userConfigs[i], destDir, token)
		}

		time.Sleep(10 * time.Second)
	}
}

func getImageTags(currConfig broker.AppUserConfigObject, destDir string, authToken string) {
	listResp, err := broker.ListGCRImageTags(currConfig.Spec.ImageRepo, authToken)
	if err != nil {
		log.Printf("failed to list image tags for: %s", currConfig.Spec.ImageRepo)
		return
	}

	// Update tags
	currConfig.Spec.Tags = listResp.Tags

	// Save app config to local file.
	if err := currConfig.WriteJSON(path.Join(destDir, broker.AppUserConfigJSONFile)); err != nil {
		log.Printf("failed to save copy of user app config: %v", err)
		return
	}
}
