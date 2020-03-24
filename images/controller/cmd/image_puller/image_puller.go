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
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	broker "gcp.solutions/kube-app-launcher/pkg"
)

const loopInterval = 10

func main() {

	log.Printf("Starting image puller")

	// Set from downward API.
	namespace := os.Getenv("NAMESPACE")
	if len(namespace) == 0 {
		log.Fatal("Missing NAMESPACE env.")
	}

	// optional polling mode.
	loop := false
	if os.Getenv("IMAGE_PULL_LOOP") == "true" {
		loop = true
	}

	// configure docker with gcloud credentials
	cmd := exec.Command("gcloud", "auth", "configure-docker", "-q")
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("failed to configure docker with gcloud: %s, %v", string(stdoutStderr), err)
	}

	for {
		images, err := findImageTags(namespace)
		if err != nil {
			log.Fatal(err)
		}

		for _, image := range images {
			log.Printf("pulling image: %s", image)
			if err := pullImage(image); err != nil {
				// non-fatal error to try pulling all images.
				log.Printf("failed to pull image: %s, %v", image, err)
			}
		}

		if loop {
			time.Sleep(loopInterval * time.Second)
		} else {
			break
		}
	}
}

// Returns de-duplicated list of image tags from:
// 1. broker apps spec.defaultRepo:defaultTag
// 2. broker apps spec.images[].newRepo:newTag structure
// 3. user config spec.ImageRepo:imageTag
func findImageTags(namespace string) ([]string, error) {
	uniqueImages := make(map[string]bool, 0)

	// Fetch all broker apps
	appConfigs, err := broker.FetchBrokerAppConfigs(namespace)
	if err != nil {
		log.Printf("failed to fetch broker app configs: %v", err)
	}

	for _, appConfig := range appConfigs {
		uniqueImages[makeImageName(appConfig.Spec.DefaultRepo, appConfig.Spec.DefaultTag)] = true
		for _, imageSpec := range appConfig.Spec.Images {
			uniqueImages[makeImageName(imageSpec.NewRepo, imageSpec.NewTag)] = true
		}
	}

	// Fetch all user app configs
	userConfigs, err := broker.FetchAppUserConfigs(namespace)
	if err != nil {
		log.Printf("failed to fetch user app configs: %v", err)
	} else {
		for _, userConfig := range userConfigs {
			uniqueImages[makeImageName(userConfig.Spec.ImageRepo, userConfig.Spec.ImageTag)] = true
		}
	}

	images := make([]string, 0)
	for image := range uniqueImages {
		images = append(images, image)
	}

	return images, nil
}

func pullImage(image string) error {
	cmd := exec.Command("docker", "pull", image)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get pods: %s, %v", string(stdoutStderr), err)
	}
	return nil
}

func makeImageName(repo, tag string) string {
	return fmt.Sprintf("%s:%s", repo, tag)
}
