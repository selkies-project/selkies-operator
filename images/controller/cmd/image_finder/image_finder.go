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
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	broker "selkies.io/controller/pkg"
)

// Interval to re-check all configs, in seconds.
const checkInterval = 60

// pubsub message receive context timeout in seconds
const pubsubRecvTimeout = 2

func main() {
	project, err := broker.GetProjectID()
	if err != nil {
		log.Fatal(err)
	}

	region, err := broker.GetInstanceRegion()
	if err != nil {
		log.Fatal(err)
	}

	// Obtain Service Account email
	saEmail, err := broker.GetServiceAccountFromMetadataServer()
	if err != nil {
		log.Fatalf("failed to get service account email: %v", err)
	}

	topicName := os.Getenv("TOPIC_NAME")
	if len(topicName) == 0 {
		topicName = "gcr"
	}

	// Perform initial check
	checkUserConfigs()

	// Subscribe to GCR pub/sub topic
	subName := fmt.Sprintf("pod-broker-image-finder-%s", region)
	sub, err := broker.GetPubSubSubscription(subName, topicName, project, saEmail)
	if err != nil {
		log.Fatal(err)
	}

	// Go routine to process all messages from subscription
	go func() {
		log.Printf("starting GCR pubsub worker")
		for {
			recvCtx, cancelRecv := context.WithTimeout(context.Background(), pubsubRecvTimeout*time.Second)
			defer cancelRecv()

			if err := sub.Receive(recvCtx, func(ctx context.Context, m *pubsub.Message) {
				var message broker.GCRPubSubMessage
				if err := json.Unmarshal(m.Data, &message); err != nil {
					log.Printf("error decoding GCR message: %v", err)
					return
				}

				if len(message.Tag) > 0 {
					// Fetch all user app configs
					userConfigs, err := broker.FetchAppUserConfigs()
					if err != nil {
						log.Fatalf("failed to fetch user app configs: %v", err)
					}

					// Update list of tags for all user configs that use this image.
					for i := range userConfigs {
						imageToks := strings.Split(message.Tag, ":")
						imageRepo := imageToks[0]
						imageTag := imageToks[1]

						if imageRepo == userConfigs[i].Spec.ImageRepo {
							log.Printf("updating list of tags for app: %s, user: %s", userConfigs[i].Spec.AppName, userConfigs[i].Spec.User)

							destDir := path.Join(broker.AppUserConfigBaseDir, userConfigs[i].Spec.AppName, userConfigs[i].Spec.User)
							err = os.MkdirAll(destDir, os.ModePerm)
							if err != nil {
								log.Fatalf("failed to create directory: %v", err)
							}

							// Update tags
							userConfigs[i].Spec.Tags = append(userConfigs[i].Spec.Tags, imageTag)

							// Save app config to local file.
							if err := userConfigs[i].WriteJSON(path.Join(destDir, broker.AppUserConfigJSONFile)); err != nil {
								log.Printf("failed to save copy of user app config: %v", err)
								return
							}
						}
					}
				} else {
					fmt.Printf("skipping gcr message because message is missing image tag: %s", message.Digest)
				}
			}); err != nil {
				fmt.Printf("error receiving message: %v", sub)
			}
		}
	}()

	log.Printf("starting user config watcher")
	for {
		// Check all user configs
		checkUserConfigs()
		time.Sleep(checkInterval * time.Second)
	}
}

func checkUserConfigs() {
	// Fetch all user app configs
	userConfigs, err := broker.FetchAppUserConfigs()
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
}

func getImageTags(currConfig broker.AppUserConfigObject, destDir string, authToken string) {
	image := fmt.Sprintf("%s:%s", currConfig.Spec.ImageRepo, currConfig.Spec.ImageTag)
	listResp, err := broker.ListGCRImageTags(image, authToken)
	if err != nil {
		log.Printf("failed to list image tags for: %s\n%v", image, err)
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
