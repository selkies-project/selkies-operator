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
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/Masterminds/sprig"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	broker "selkies.io/controller/pkg"
)

// job cleanup loop interval in seconds
const jobCleanupInterval = 10

// new image check interval in seconds
const newImagePullInterval = 300

// duration between informer resyncs
const resyncDuration = 60 * time.Second

// duration between node image listings
const nodeImageUpdateInterval = 5 * time.Second

// Synchronous slice of de-duplicated images to process
type NodeImagesSync struct {
	sync.Mutex
	Images []broker.DockerImage
}

func (ni *NodeImagesSync) Update() {
	ni.Lock()
	defer ni.Unlock()
	nodeImages, err := broker.GetImagesOnNode()
	if err != nil {
		log.Printf("error getting images on node: %v", err)
	}
	ni.Images = nodeImages
}

func (ni *NodeImagesSync) Contains(repo, imageDigest string) bool {
	ni.Lock()
	defer ni.Unlock()
	for _, nodeImage := range ni.Images {
		if repo == nodeImage.Repository && imageDigest == nodeImage.Digest {
			return true
		}
	}
	return false
}

func (ni *NodeImagesSync) Len() int {
	ni.Lock()
	defer ni.Unlock()
	return len(ni.Images)
}

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

	// Values available to templates from environment variables prefixed with POD_BROKER_PARAM_Name=Value
	// Map of Name=Value
	sysParams := broker.GetEnvPrefixedVars("POD_BROKER_PARAM_")

	workerImage := fmt.Sprintf("gcr.io/%s/kube-pod-broker-controller:latest", project)
	if workerImageParam, ok := sysParams["ImagePullerWorkerImage"]; ok {
		workerImage = workerImageParam
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

	// Get docker config pull secrets
	dockerConfigs := &broker.DockerConfigsSync{}
	if err := dockerConfigs.Update(namespace); err != nil {
		log.Fatalf("failed to fetch docker auth configs: %v", err)
	}

	// Go routine to watch for pull secrets using informer.
	go func() {
		// TODO: Refactor this into informer and add separate goroutine for updating the default SA token.
		for {
			if err := dockerConfigs.Update(namespace); err != nil {
				log.Printf("failed to update docker auth configs: %v", err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	// Go routine to periodically update the list of docker images on the node.
	nodeImages := &NodeImagesSync{}
	nodeImages.Update()
	go func() {
		for {
			time.Sleep(nodeImageUpdateInterval)
			nodeImages.Update()
		}
	}()

	// Initialize synchronous slice of images to process
	imageQueue := broker.NewImageQueueSync()

	// Initialize synchronous slice of images found so we can filter the pubsub messages.
	knownImages := broker.NewImageQueueSync()

	processImage := func(image string) {
		// Fetch image details.
		repo, err := broker.GetDockerRepoFromImage(image)
		if err != nil {
			log.Printf("failed to get image repo from image: %s: %v", image, err)
			return
		}
		imageDigest, err := dockerConfigs.GetDigest(image)
		if err != nil {
			log.Printf("error fetching image digest: %s, %v", image, err)
			return
		}

		// Keep track of all relavent images found so we can filter the pubsub messages.
		knownImages.Push(image)

		// Check if image is already on node.
		if nodeImages.Len() == 0 {
			log.Printf("WARN: no node images found.")
			return
		}
		imageOnNode := nodeImages.Contains(repo, imageDigest)

		// Pull image if not found.
		if !imageOnNode {
			imageWithDigest := fmt.Sprintf("%s@%s", repo, imageDigest)
			imageTag, err := broker.GetDockerTagFromImage(image)
			if err != nil {
				log.Printf("%v", err)
			} else {
				dockerConfigJSON, err := dockerConfigs.GetDockerConfigJSONForRepo(image)
				if err != nil {
					log.Printf("could not find valid docker auth config for image: %s: %v", image, err)
				} else {
					log.Printf("creating image pull job for: %s", imageWithDigest)
					if err := pullImage(imageWithDigest, imageTag, namespace, nodeName, templatePath, dockerConfigJSON, workerImage); err != nil {
						log.Printf("%v", err)
					}
				}
			}
		}
	}

	// Watch changes to BrokerAppConfigs with informer.
	addAppConfigFunc := func(obj broker.AppConfigObject) {
		// log.Printf("Saw new BrokerAppUserConfig: %s", obj.Metadata.Name)
		// Add default image
		imageQueue.Push(makeImageName(obj.Spec.DefaultRepo, obj.Spec.DefaultTag))

		// Add related images
		for _, imageSpec := range obj.Spec.Images {
			imageQueue.Push(makeImageName(imageSpec.NewRepo, imageSpec.NewTag))
		}
	}
	deleteAppConfigFunc := func(obj broker.AppConfigObject) {
		// log.Printf("Saw deletion of BrokerAppUserConfig: %s", obj.Metadata.Name)
		// Remove default image
		imageQueue.Remove(makeImageName(obj.Spec.DefaultRepo, obj.Spec.DefaultTag))

		// Remove related images
		for _, imageSpec := range obj.Spec.Images {
			imageQueue.Remove(makeImageName(imageSpec.NewRepo, imageSpec.NewTag))
		}
	}
	updateAppConfigFunc := func(oldObj, newObj broker.AppConfigObject) {
		// log.Printf("Saw update for BrokerAppUserConfig: %s", newObj.Metadata.Name)
		// Remove old default image
		imageQueue.Remove(makeImageName(oldObj.Spec.DefaultRepo, oldObj.Spec.DefaultTag))
		// Add new default image
		imageQueue.Push(makeImageName(newObj.Spec.DefaultRepo, newObj.Spec.DefaultTag))

		// Remove old related images
		for _, imageSpec := range oldObj.Spec.Images {
			imageQueue.Remove(makeImageName(imageSpec.NewRepo, imageSpec.NewTag))
		}

		// Add new releated images
		for _, imageSpec := range newObj.Spec.Images {
			imageQueue.Push(makeImageName(imageSpec.NewRepo, imageSpec.NewTag))
		}
	}
	appConfigInformer := broker.NewAppConfigInformer(addAppConfigFunc, deleteAppConfigFunc, updateAppConfigFunc)
	appConfigStopper := make(chan struct{})
	defer close(appConfigStopper)
	informerOpts := &broker.PodBrokerInformerOpts{
		ResyncDuration: resyncDuration,
		ClientConfig:   config,
	}
	if err := broker.RunPodBrokerInformer(appConfigInformer, appConfigStopper, informerOpts); err != nil {
		log.Fatalf("Error starting BrokerAppConfig informer: %v", err)
	}

	// Watch changes to BrokerAppUserConfigs with informer.
	addUserConfigFunc := func(obj broker.AppUserConfigObject) {
		// log.Printf("Saw new BrokerAppUserConfig: %s", obj.Metadata.Name)
		imageQueue.Push(makeImageName(obj.Spec.ImageRepo, obj.Spec.ImageTag))
	}
	deleteUserConfigFunc := func(obj broker.AppUserConfigObject) {
		// log.Printf("Saw deletion of BrokerAppUserConfig: %s", obj.Metadata.Name)
		imageQueue.Remove(makeImageName(obj.Spec.ImageRepo, obj.Spec.ImageTag))
	}
	updateUserConfigFunc := func(oldObj, newObj broker.AppUserConfigObject) {
		// log.Printf("Saw update for BrokerAppUserConfig: %s", newObj.Metadata.Name)
		imageQueue.Remove(makeImageName(oldObj.Spec.ImageRepo, oldObj.Spec.ImageTag))
		imageQueue.Push(makeImageName(newObj.Spec.ImageRepo, newObj.Spec.ImageTag))
	}
	userConfigInformer := broker.NewAppUserConfigInformer(addUserConfigFunc, deleteUserConfigFunc, updateUserConfigFunc)
	userConfigStopper := make(chan struct{})
	defer close(userConfigStopper)
	if err := broker.RunPodBrokerInformer(userConfigInformer, userConfigStopper, informerOpts); err != nil {
		log.Fatalf("Error starting BrokerAppUserConfig informer: %v", err)
	}

	// Print initial list of images.
	fmt.Printf("Initial images to watch:\n")
	imageQueue.Lock()
	for _, img := range imageQueue.ImageQueue {
		fmt.Printf("  %s\n", img)
	}
	if v := os.Getenv("LIST_IMAGES_ONLY"); v == "true" {
		log.Printf("Exiting")
		os.Exit(0)
	}
	imageQueue.Unlock()

	// Subscribe to GCR pub/sub topic
	subName := fmt.Sprintf("pod-broker-image-puller-%s", nodeName)
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
		log.Printf("starting GCR pubsub worker on subscription: %s", subName)
		var mu sync.Mutex
		for {
			recvCtx, cancelRecv := context.WithCancel(context.Background())
			defer cancelRecv()

			log.Printf("pulling messages")
			if err := sub.Receive(recvCtx, func(ctx context.Context, m *pubsub.Message) {
				defer m.Ack()
				mu.Lock()
				defer mu.Unlock()

				var message broker.GCRPubSubMessage
				if err := json.Unmarshal(m.Data, &message); err != nil {
					log.Printf("error decoding GCR message: %v", err)
					return
				}

				imageWithTag := message.Tag

				if message.Action == "DELETE" {
					log.Printf("image deleted: %v", message)
					if len(imageWithTag) > 0 {
						imageQueue.Remove(imageWithTag)
						knownImages.Remove(imageWithTag)
					}
					return
				}

				if len(imageWithTag) > 0 && knownImages.Contains(imageWithTag) {
					log.Printf("queuing image from pub/sub: %s", imageWithTag)
					imageQueue.Push(imageWithTag)
				} else {
					log.Printf("skipping pubsub message because image '%s' was invalid or not related to any broker app.", imageWithTag)
				}
				time.Sleep(100 * time.Millisecond)
			}); err != nil {
				fmt.Printf("error receiving message: %v", err)
			}
			time.Sleep(2 * time.Second)
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

	// Start the image queue worker
	log.Printf("starting image queue worker")
	for {
		if imageQueue.Len() > 0 {
			for {
				image := imageQueue.Pop()
				if len(image) == 0 {
					break
				}
				processImage(image)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Creates Job to pull image if one is not already running.
func pullImage(imageWithDigest, imageTag, namespace, nodeName, templatePath, dockerConfigJSON, workerImage string) error {
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
		if err := makeImagePullJob(imageWithDigest, imageTag, nodeName, namespace, templatePath, dockerConfigJSON, workerImage); err != nil {
			return fmt.Errorf("failed to make job: %v", err)
		}
	}
	return nil
}

func makeImageName(repo, tag string) string {
	return fmt.Sprintf("%s:%s", repo, tag)
}

// Check if job is currently running.
// If running, return (non-fatal) error.
// If not running, apply job to given namespace.
func makeImagePullJob(image, tag, nodeName, namespace, templatePath, dockerConfigJSON, workerImage string) error {
	imageRepo, err := broker.GetDockerRepoFromImage(image)
	if err != nil {
		return err
	}
	imageToks := strings.Split(image, "@sha256:")
	if len(imageToks) < 2 {
		return fmt.Errorf("missing @sha256: in image: %s", image)
	}
	digestHash := imageToks[1]
	dockerConfigJSON64 := base64.StdEncoding.EncodeToString([]byte(dockerConfigJSON))

	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("%s", nodeName))
	nodeNameHash := fmt.Sprintf("%x", h.Sum(nil))

	imageRepoSlug := strings.ReplaceAll(imageRepo, "/", "-")
	if len(imageRepoSlug) > 40 {
		imageRepoSlug = imageRepoSlug[:40]
	}

	nameSuffix := fmt.Sprintf("%s-%s-%s", imageRepoSlug, digestHash[:5], nodeNameHash[:5])
	type templateData struct {
		NameSuffix         string
		NodeName           string
		Image              string
		Tag                string
		DockerConfigJSON64 string
		WorkerImage        string
	}

	data := templateData{
		NameSuffix:         nameSuffix,
		NodeName:           nodeName,
		Image:              image,
		Tag:                tag,
		DockerConfigJSON64: dockerConfigJSON64,
		WorkerImage:        workerImage,
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
