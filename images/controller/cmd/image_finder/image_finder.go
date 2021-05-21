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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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

	// Perform initial check
	checkUserConfigs()

	// Go routine to watch BrokerAppUserConfigs
	go func() {
		log.Printf("starting user config informer")
		myinformer, err := GetDynamicInformer(config, "brokerappuserconfigs.v1.gcp.solutions")
		if err != nil {
			log.Printf("%v", err)
			return
		}
		stopper := make(chan struct{})
		defer close(stopper)
		runCRDInformer(stopper, myinformer.Informer(), namespace)
	}()

	// Subscribe to GCR pub/sub topic
	subName := fmt.Sprintf("pod-broker-image-finder-%s", nodeName)
	var sub *pubsub.Subscription

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

				if message.Action == "DELETE" {
					log.Printf("image deleted: %v", message)
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
		checkUserConfigs()
		time.Sleep(checkInterval * time.Second)
	}
}

func writeUserConfigJSON(userConfig broker.AppUserConfigObject) error {
	destDir := path.Join(broker.AppUserConfigBaseDir, userConfig.Spec.AppName, userConfig.Spec.User)
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Get service account name from metadata server
	sa, err := broker.GetServiceAccountFromMetadataServer()
	if err != nil {
		return fmt.Errorf("failed to get service account name from metadata server: %v", err)
	}

	// Get access token from metadata server
	token, err := broker.GetServiceAccountTokenFromMetadataServer(sa)
	if err != nil {
		return fmt.Errorf("failed to get token from metadata server: %v", err)
	}

	if len(userConfig.Spec.ImageRepo) > 0 && len(userConfig.Spec.ImageTag) > 0 {
		// Fill in the userConfig with the list of tags from GCR, then write the JSON to a file.
		getImageTags(userConfig, destDir, token)
	} else {
		// Save app config to local file.
		if err := userConfig.WriteJSON(path.Join(destDir, broker.AppUserConfigJSONFile)); err != nil {
			return fmt.Errorf("failed to save copy of user app config: %v", err)
		}
	}

	return nil
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

func makeUserConfigFromUnstructured(obj interface{}) (broker.AppUserConfigObject, error) {
	d := broker.AppUserConfigObject{}
	err := runtime.DefaultUnstructuredConverter.
		FromUnstructured(obj.(*unstructured.Unstructured).UnstructuredContent(), &d)
	if err != nil {
		return d, err
	}
	d.ApiVersion = broker.ApiVersion
	d.Kind = broker.BrokerAppUserConfigKind
	return d, nil
}

func runCRDInformer(stopCh <-chan struct{}, s cache.SharedIndexInformer, namespace string) {
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			d, err := makeUserConfigFromUnstructured(obj)
			if err != nil {
				fmt.Printf("could not convert obj: %v", err)
				return
			}
			log.Printf("Saw new BrokerAppUserConfig: %s", d.Metadata.Name)

			if err := writeUserConfigJSON(d); err != nil {
				log.Printf("failed to save user config to JSON: %v", err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			d, err := makeUserConfigFromUnstructured(obj)
			if err != nil {
				fmt.Printf("could not convert obj: %v", err)
				return
			}
			log.Printf("Saw deletion of BrokerAppUserConfig: %s", d.Metadata.Name)

			destDir := path.Join(broker.AppUserConfigBaseDir, d.Spec.AppName, d.Spec.User)
			destFile := path.Join(destDir, broker.AppUserConfigJSONFile)
			os.Remove(destFile)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			d, err := makeUserConfigFromUnstructured(newObj)
			if err != nil {
				fmt.Printf("could not convert obj: %v", err)
				return
			}
			log.Printf("Saw update for BrokerAppUserConfig: %s", d.Metadata.Name)

			if err := writeUserConfigJSON(d); err != nil {
				log.Printf("failed to save user config to JSON: %v", err)
			}
		},
	}
	s.AddEventHandler(handlers)
	s.Run(stopCh)
}

func GetDynamicInformer(cfg *rest.Config, resourceType string) (informers.GenericInformer, error) {
	// Grab a dynamic interface that we can create informers from
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	// Create a factory object that can generate informers for resource types
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc, 0, corev1.NamespaceAll, nil)
	// "GroupVersionResource" to say what to watch e.g. "deployments.v1.apps" or "seldondeployments.v1.machinelearning.seldon.io"
	gvr, _ := schema.ParseResourceArg(resourceType)
	// Finally, create our informer for deployments!
	informer := factory.ForResource(*gvr)
	return informer, nil
}
