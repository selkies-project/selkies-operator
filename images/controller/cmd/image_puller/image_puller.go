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
	"crypto/sha1"
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

	broker "gcp.solutions/kube-app-launcher/pkg"
	"github.com/Masterminds/sprig"
)

const loopInterval = 2

func main() {

	log.Printf("Starting image puller")

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

	// Obtain SA email
	saEmail, err := broker.GetServiceAccountFromMetadataServer()
	if err != nil {
		log.Fatalf("failed to get service account email: %v", err)
	}

	var wg sync.WaitGroup

	for {
		token, err := broker.GetServiceAccountTokenFromMetadataServer(saEmail)
		if err != nil {
			log.Fatalf("failed to get service account token: %v", err)
			if loop {
				time.Sleep(loopInterval * time.Second)
			} else {
				break
			}
		}

		images, err := findImageTags(namespace)
		if err != nil {
			log.Fatal(err)
		}

		nodeImages, err := broker.GetImagesOnNode()
		if err != nil {
			log.Fatal(err)
		}

		currJobs, err := broker.GetJobs(namespace, "app=image-pull")
		if err != nil {
			log.Fatal(err)
		}

		// Process images in parallel
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
							// Check to see if job is active.
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
										log.Printf("missing pod.broker/image-pull annotation on job")
									}
								} else {
									log.Printf("failed to get job annotations")
								}
							}

							if !jobFound {
								if err := makeImagePullJob(imageWithDigest, nodeName, namespace, templatePath); err != nil {
									log.Printf("failed to make job: %v", err)
								}
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

	imageTags, err := broker.ListGCRImageTags(image, accessToken)
	if err != nil {
		return respImage, err
	}

	if len(regexp.MustCompile(broker.GCRImageWithTagPattern).FindAllString(image, -1)) > 0 {
		// Find image digest from tag.
		imageToks := strings.Split(strings.ReplaceAll(image, "gcr.io/", ""), ":")
		imageRepo := imageToks[0]
		imageTag := imageToks[1]

		for digest, meta := range imageTags.Manifest {
			for _, tag := range meta.Tag {
				if tag == imageTag {
					respImage = fmt.Sprintf("gcr.io/%s@%s", imageRepo, digest)
					break
				}
			}
			if len(respImage) > 0 {
				break
			}
		}
	}

	if len(regexp.MustCompile(broker.GCRImageWithDigestPattern).FindAllString(image, -1)) > 0 {
		// Verify image digest is in list response.
		imageDigest := strings.Split(strings.ReplaceAll(image, "gcr.io/", ""), "@")[1]
		if _, ok := imageTags.Manifest[imageDigest]; ok {
			respImage = image
		}
	}

	if len(respImage) == 0 {
		return respImage, fmt.Errorf("failed to find digest for image: %s", image)
	}

	return respImage, nil
}

// Check if job is currently running.
// If running, return (non-fatal) error.
// If not running, apply job to given namespace.
func makeImagePullJob(image, nodeName, namespace, templatePath string) error {

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
	}

	data := templateData{
		NameSuffix: nameSuffix,
		NodeName:   nodeName,
		Image:      image,
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
