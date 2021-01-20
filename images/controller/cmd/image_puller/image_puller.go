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
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/Masterminds/sprig"
	broker "selkies.io/controller/pkg"
)

// pubsub message receive context timeout in seconds
const pubsubRecvTimeout = 2

// job cleanup loop interval in seconds
const jobCleanupInterval = 10

// new image check interval in seconds
const newImagePullInterval = 5

func main() {

	log.Printf("starting image puller")

	// Set from downward API.
	namespace := os.Getenv("NAMESPACE")
	if len(namespace) == 0 {
		log.Fatal("Missing NAMESPACE env.")
	}

	// Set from downward API.
	nodeName := os.Getenv("NODE_NAME")
	if len(nodeName) == 0 {
		log.Fatal("Missing NODE_NAME env.")
	}

	templatePath := os.Getenv("TEMPLATE_PATH")
	if len(templatePath) == 0 {
		templatePath = "/run/image-puller/template/image-pull-job.yaml.tmpl"
	}

	// configure docker with gcloud credentials
	cmd := exec.Command("gcloud", "auth", "configure-docker", "-q")
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("failed to configure docker with gcloud: %s, %v", string(stdoutStderr), err)
	}

	// Obtain Service Account email
	saEmail, err := broker.GetServiceAccountFromMetadataServer()
	if err != nil {
		log.Fatalf("failed to get service account email: %v", err)
	}

	project, err := broker.GetProjectID()
	if err != nil {
		log.Fatal(err)
	}

	topicName := os.Getenv("TOPIC_NAME")
	if len(topicName) == 0 {
		topicName = "gcr"
	}

	// Subscribe to GCR pub/sub topic
	subName := fmt.Sprintf("pod-broker-image-puller-%s", nodeName)
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
					// Fetch list of current images we care about.
					images, err := findImageTags(namespace)
					if err != nil {
						log.Fatal(err)
					}

					imageWithTag := message.Tag
					foundMatchingImage := false
					for _, image := range images {
						if image == imageWithTag {
							foundMatchingImage = true
							break
						}
					}
					if foundMatchingImage {
						imageWithDigest := message.Digest
						imageToks := strings.Split(imageWithTag, ":")
						imageTag := imageToks[1]

						// Check to see if image is already on node.
						nodeImages, err := broker.GetImagesOnNode()
						if err != nil {
							log.Fatal(err)
						}

						// Check if image is already on node.
						imageOnNode := false
						for _, nodeImage := range nodeImages {
							if fmt.Sprintf("%s@%s", nodeImage.Repository, nodeImage.Digest) == imageWithDigest {
								imageOnNode = true
							}
						}

						if !imageOnNode {
							if err := pullImage(imageWithDigest, imageTag, namespace, nodeName, templatePath); err != nil {
								log.Printf("%v", err)
							}
						}
					} else {
						fmt.Printf("skipping image pull because image is not used by any apps: %s", imageWithTag)
					}
				} else {
					fmt.Printf("skipping gcr message because message is missing image tag: %s", message.Digest)
				}
				m.Ack()
			}); err != nil {
				fmt.Printf("error receiving message: %v", sub)
			}
		}
	}()

	// Go routine to cleanup completed jobs.
	go func() {
		log.Printf("starting job cleanup worker")
		for {
			currJobs, err := broker.GetJobs(namespace, "app=image-pull")
			if err != nil {
				log.Fatalf("failed to get current jobs: %v", err)
			}

			// Delete completed jobs.
			for _, job := range currJobs {
				if metaValue, ok := job.Metadata["annotations"]; ok {
					annotations := metaValue.(map[string]interface{})
					if imagePullAnnotation, ok := annotations["pod.broker/image-pull"]; ok {
						if strings.Split(imagePullAnnotation.(string), ",")[0] == nodeName {
							// Found job for node.
							jobName := job.Metadata["name"].(string)
							if job.Status.Succeeded > 0 {
								log.Printf("deleting completed job: %s", jobName)
								cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl delete job -n %s %s 1>&2", namespace, jobName))
								stdoutStderr, err := cmd.CombinedOutput()
								if err != nil {
									log.Printf("error calling kubectl to delete job: %v\n%s", err, string(stdoutStderr))
								}
							}
						}
					} else {
						log.Printf("missing pod.broker/image-pull annotation on job")
					}
				}
			}
			time.Sleep(jobCleanupInterval * time.Second)
		}
	}()

	// Go routine to fetch new images
	log.Printf("starting new image puller worker")
	for {
		// Perform one-time list of all image tags then exit.
		token, err := broker.GetServiceAccountTokenFromMetadataServer(saEmail)
		if err != nil {
			log.Fatalf("failed to get service account token: %v", err)
		}

		images, err := findImageTags(namespace)
		if err != nil {
			log.Fatal(err)
		}

		nodeImages, err := broker.GetImagesOnNode()
		if err != nil {
			log.Fatal(err)
		}

		// Process images in parallel
		var wg sync.WaitGroup
		wg.Add(len(images))
		for _, image := range images {
			go func(image, token string) {
				// Fetch image details for images in the form of: "gcr.io.*:tag"
				if image[:6] == "gcr.io" {
					imageWithDigest, err := getImageDigest(image, token)
					if err != nil {
						log.Printf("error fetching image digest: %s, %v", image, err)
					} else {
						// Check if image is already on node.
						imageOnNode := false
						for _, nodeImage := range nodeImages {
							if fmt.Sprintf("%s@%s", nodeImage.Repository, nodeImage.Digest) == imageWithDigest {
								imageOnNode = true
							}
						}

						if !imageOnNode {
							imageTag := getTagFromImage(image)
							if err := pullImage(imageWithDigest, imageTag, namespace, nodeName, templatePath); err != nil {
								log.Printf("%v", err)
							}
						}
					}
				} else {
					// Non-gcr image
					log.Printf("skipping pull of non-gcr image: %s", image)
				}
				wg.Done()
			}(image, token)
		}
		wg.Wait()

		time.Sleep(newImagePullInterval * time.Second)
	}
}

// Creates Job to pull image if one is not already running.
func pullImage(imageWithDigest, imageTag, namespace, nodeName, templatePath string) error {
	// Check to see if job is active.
	currJobs, err := broker.GetJobs(namespace, "app=image-pull")
	if err != nil {
		return err
	}

	jobFound := false
	for _, job := range currJobs {
		if metaValue, ok := job.Metadata["annotations"]; ok {
			annotations := metaValue.(map[string]interface{})
			if imagePullAnnotation, ok := annotations["pod.broker/image-pull"]; ok {
				if imagePullAnnotation.(string) == fmt.Sprintf("%s,%s", nodeName, imageWithDigest) {
					// Found a job with matching image digest.
					jobFound = true
				}
			} else {
				return fmt.Errorf("missing pod.broker/image-pull annotation on job")
			}
		} else {
			return fmt.Errorf("failed to get job annotations")
		}
	}

	if !jobFound {
		log.Printf("creating image pull job for %s", imageWithDigest)
		if err := makeImagePullJob(imageWithDigest, imageTag, nodeName, namespace, templatePath); err != nil {
			return fmt.Errorf("failed to make job: %v", err)
		}
	}
	return nil
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
	userConfigs, err := broker.FetchAppUserConfigs()
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

func makeImageName(repo, tag string) string {
	return fmt.Sprintf("%s:%s", repo, tag)
}

// Find and verify image digest
func getImageDigest(image, accessToken string) (string, error) {
	respImage := ""

	if len(regexp.MustCompile(broker.GCRImageWithTagPattern).FindAllString(image, -1)) > 0 {
		// Find image digest from tag.
		imageToks := strings.Split(strings.ReplaceAll(image, "gcr.io/", ""), ":")
		imageRepo := imageToks[0]
		imageTag := imageToks[1]

		digest, err := broker.GetGCRDigestFromTag(imageRepo, imageTag, accessToken)
		if err != nil {
			return respImage, err
		}

		respImage = fmt.Sprintf("gcr.io/%s@%s", imageRepo, digest)
	}

	if len(regexp.MustCompile(broker.GCRImageWithDigestPattern).FindAllString(image, -1)) > 0 {
		// Verify image digest is in list response.
		imageToks := strings.Split(strings.ReplaceAll(image, "gcr.io/", ""), "@")
		imageRepo := imageToks[0]
		imageDigest := imageToks[1]
		digest, err := broker.GetGCRDigestFromTag(imageRepo, imageDigest, accessToken)
		if err != nil {
			return respImage, err
		}
		if imageDigest == digest {
			respImage = image
		}
	}

	if len(respImage) == 0 {
		return respImage, fmt.Errorf("failed to find digest for image: %s", image)
	}

	return respImage, nil
}

// Extract tag from image repo:tag format, else return empty string.
func getTagFromImage(image string) string {
	if len(regexp.MustCompile(broker.GCRImageWithTagPattern).FindAllString(image, -1)) > 0 {
		return strings.Split(strings.ReplaceAll(image, "gcr.io/", ""), ":")[1]
	}
	return ""
}

// Check if job is currently running.
// If running, return (non-fatal) error.
// If not running, apply job to given namespace.
func makeImagePullJob(image, tag, nodeName, namespace, templatePath string) error {

	imageToks := strings.Split(strings.ReplaceAll(image, "gcr.io/", ""), "@sha256:")
	imageBase := path.Base(imageToks[0])
	digestHash := imageToks[1]

	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%s", nodeName))
	nodeNameHash := fmt.Sprintf("%x", h.Sum(nil))

	nameSuffix := fmt.Sprintf("%s-%s-%s", imageBase, digestHash[:5], nodeNameHash[:5])

	log.Printf("creating image pull job: %s, %s, %s", nodeName, image, nameSuffix)

	type templateData struct {
		NameSuffix string
		NodeName   string
		Image      string
		Tag        string
	}

	data := templateData{
		NameSuffix: nameSuffix,
		NodeName:   nodeName,
		Image:      image,
		Tag:        tag,
	}

	destDir := path.Join("/run/image-puller", nameSuffix)
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to make destDir %s: %v", destDir, err)
	}

	base := path.Base(templatePath)
	t, err := template.New(base).Funcs(sprig.TxtFuncMap()).ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to initialize template: %v", err)
	}
	dest, _ := os.Create(strings.ReplaceAll(path.Join(destDir, base), ".tmpl", ""))
	if err != nil {
		return fmt.Errorf("failed to create dest template file: %v", err)
	}
	if err = t.Execute(dest, &data); err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	// Apply the job to the cluster
	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl apply -f %s 1>&2", destDir))
	cmd.Dir = path.Dir(destDir)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error calling kubectl to apply job: %v\n%s", err, string(stdoutStderr))
	}

	return nil
}
