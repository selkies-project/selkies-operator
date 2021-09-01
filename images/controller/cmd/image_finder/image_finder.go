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
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"cloud.google.com/go/pubsub"
	broker "selkies.io/controller/pkg"
)

// Interval to re-check all configs, in seconds.
const checkInterval = 300

// pubsub message receive context timeout in seconds
const pubsubRecvTimeout = 2

func main() {
	// Sleep if disabled via env
	if enabledEnv := os.Getenv("POD_BROKER_PARAM_EnableImagePuller"); enabledEnv == "false" {
		log.Printf("Image finder disabled via env param POD_BROKER_PARAM_EnableImagePuller, sleeping")
		for {
			time.Sleep(1000 * time.Second)
		}
	}
	project, err := broker.GetProjectID()
	if err != nil {
		log.Fatal(err)
	}

	namespace := os.Getenv("NAMESPACE")
	if len(namespace) == 0 {
		namespace = "pod-broker-system"
		log.Printf("Missing NAMESPACE env, using: %s", namespace)
	}

	// Set from downward API.
	nodeName := os.Getenv("NODE_NAME")
	if len(nodeName) == 0 {
		log.Fatal("Missing NODE_NAME env.")
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

	// Flags used for connecting out-of-cluster.
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// Try in-cluster config
	config, err := rest.InClusterConfig()
	if err == nil {
		log.Printf("using in-cluster-config")
	} else {
		// Try out-of-cluster config
		outOfClusterConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
		log.Printf("using out-of-cluster-config")
		config = outOfClusterConfig
	}

	dockerConfigs := &broker.DockerConfigsSync{}
	if err := dockerConfigs.Update(namespace); err != nil {
		log.Fatalf("failed to fetch docker auth configs: %v", err)
	}

	// Go routine to watch for pull secrets using dynamic informer.
	go func() {
		// TODO: Refactor this into informer and add separate goroutine for updating the default SA token.
		for {
			if err := dockerConfigs.Update(namespace); err != nil {
				log.Printf("failed to update docker auth configs: %v", err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	// Perform initial check
	checkUserConfigs(dockerConfigs)

	// Watch BrokerAppUserConfigs with dynamic informer.
	addFunc := func(obj broker.AppUserConfigObject) {
		log.Printf("Saw new BrokerAppUserConfig: %s", obj.Metadata.Name)
		if err := writeUserConfigJSON(obj, dockerConfigs); err != nil {
			log.Printf("failed to save user config to JSON: %v", err)
		}
	}
	deleteFunc := func(obj broker.AppUserConfigObject) {
		log.Printf("Saw deletion of BrokerAppUserConfig: %s", obj.Metadata.Name)
		destDir := path.Join(broker.AppUserConfigBaseDir, obj.Spec.AppName, obj.Spec.User)
		destFile := path.Join(destDir, broker.AppUserConfigJSONFile)
		os.Remove(destFile)
	}
	updateFunc := func(oldObj, newObj broker.AppUserConfigObject) {
		log.Printf("Saw update for BrokerAppUserConfig: %s", newObj.Metadata.Name)
		if err := writeUserConfigJSON(newObj, dockerConfigs); err != nil {
			log.Printf("failed to save user config to JSON: %v", err)
		}
	}
	informer := broker.NewAppUserConfigInformer(addFunc, deleteFunc, updateFunc)
	go func() {
		stopper := make(chan struct{})
		defer close(stopper)
		opts := &broker.PodBrokerInformerOpts{
			ResyncDuration: 0,
			ClientConfig:   config,
		}
		broker.RunPodBrokerInformer(informer, stopper, opts)
	}()

	// Subscribe to GCR pub/sub topic
	var sub *pubsub.Subscription
	subName := fmt.Sprintf("pod-broker-image-finder-%s", nodeName)

	// Poll until subscription is obtained
	for {
		sub, err = broker.GetPubSubSubscription(subName, topicName, project, saEmail)
		if err != nil {
			log.Printf("error getting subscription for topic %s: %v", topicName, err)
		} else {
			break
		}
		time.Sleep(2 * time.Minute)
	}

	// Go routine to process all messages from subscription
	go func() {
		log.Printf("starting GCR pubsub worker")
		var mu sync.Mutex
		for {
			cctx, cancelRecv := context.WithTimeout(context.Background(), pubsubRecvTimeout*time.Second)
			defer cancelRecv()

			if err := sub.Receive(cctx, func(ctx context.Context, m *pubsub.Message) {
				defer m.Ack()
				mu.Lock()
				defer mu.Unlock()

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
						appName := userConfigs[i].Spec.AppName
						user := userConfigs[i].Spec.User
						imageToks := strings.Split(message.Tag, ":")
						imageRepo := imageToks[0]
						imageTag := imageToks[1]
						currTags := userConfigs[i].Spec.Tags

						// Read local file to get up to date tags list.
						userConfig, err := broker.GetAppUserConfig(path.Join(broker.AppUserConfigBaseDir, appName, user, broker.AppUserConfigJSONFile))
						if err == nil {
							currTags = userConfig.Spec.Tags
						}

						if imageRepo == userConfigs[i].Spec.ImageRepo {
							destDir := path.Join(broker.AppUserConfigBaseDir, appName, user)
							err = os.MkdirAll(destDir, os.ModePerm)
							if err != nil {
								log.Fatalf("failed to create directory: %v", err)
							}

							if message.Action == "INSERT" {
								// Add tag to user config
								log.Printf("adding tag to app %s for user %s: %s", appName, user, message.Tag)
								currTags = append(currTags, imageTag)
							} else if message.Action == "DELETE" {
								// remove tag from config
								log.Printf("deleting tag from app %s for user %s: %s", appName, user, message.Tag)
								newTags := make([]string, 0)
								for _, tag := range currTags {
									if tag != imageTag {
										newTags = append(newTags, tag)
									}
								}
								currTags = newTags
							}

							userConfigs[i].Spec.Tags = currTags

							if len(currTags) == 0 {
								// image deleted
								userConfigs[i].Spec.ImageRepo = ""
								userConfigs[i].Spec.ImageTag = ""
							}

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
				time.Sleep(100 * time.Millisecond)
			}); err != nil {
				fmt.Printf("error receiving message: %v\n", err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	log.Printf("starting user config refresher")
	for {
		// Check all user configs
		checkUserConfigs(dockerConfigs)
		time.Sleep(checkInterval * time.Second)
	}
}

func writeUserConfigJSON(userConfig broker.AppUserConfigObject, dockerConfigs *broker.DockerConfigsSync) error {
	destDir := path.Join(broker.AppUserConfigBaseDir, userConfig.Spec.AppName, userConfig.Spec.User)
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Fill in the userConfig with the list of tags from GCR, then write the JSON to a file.
	getImageTags(userConfig, destDir, dockerConfigs)

	// Save app config to local file.
	if err := userConfig.WriteJSON(path.Join(destDir, broker.AppUserConfigJSONFile)); err != nil {
		return fmt.Errorf("failed to save copy of user app config: %v", err)
	}

	return nil
}

func checkUserConfigs(dockerConfigs *broker.DockerConfigsSync) {
	// Fetch all user app configs
	userConfigs, err := broker.FetchAppUserConfigs()
	if err != nil {
		log.Fatalf("failed to fetch user app configs: %v", err)
	}

	// Discover image tags in parallel for all app specs.
	for i := range userConfigs {
		destDir := path.Join(broker.AppUserConfigBaseDir, userConfigs[i].Spec.AppName, userConfigs[i].Spec.User)
		err = os.MkdirAll(destDir, os.ModePerm)
		if err != nil {
			log.Fatalf("failed to create directory: %v", err)
		}

		// Fetch in go routine.
		go getImageTags(userConfigs[i], destDir, dockerConfigs)
	}
}

func getImageTags(currConfig broker.AppUserConfigObject, destDir string, dc *broker.DockerConfigsSync) {
	image := fmt.Sprintf("%s:%s", currConfig.Spec.ImageRepo, currConfig.Spec.ImageTag)
	tags, err := dc.ListTags(image)
	if err != nil {
		log.Printf("failed to list image tags for: %s\n%v", image, err)
		return
	}

	// Update tags
	currConfig.Spec.Tags = tags

	// Save app config to local file.
	if err := currConfig.WriteJSON(path.Join(destDir, broker.AppUserConfigJSONFile)); err != nil {
		log.Printf("failed to save copy of user app config: %v", err)
		return
	}
}
