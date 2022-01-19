package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type statMetadataSpec struct {
	Name string `json:"name"`
	Node string `json:"node"`
	Tier string `json:"tier"`
}

type statProgressSpec struct {
	Image        string `json:"image"`
	PullProgress int    `json:"pull_progress"`
}

type statSpec struct {
	Metadta statMetadataSpec `json:"metadata"`
	Stats   statProgressSpec `json:"stats"`
}

const (
	timeout       = 30 * time.Minute
	statsInterval = 1 * time.Second
)

var (
	image         = flag.String("image", "", "image name and tag to pull")
	statsEndpoint = flag.String("stats", "", "endpoint to POST progress stats to")
	jobName       = flag.String("jobName", "", "job name to use when POSTing stats")
	nodeName      = flag.String("nodeName", "", "name of node the image is being pulled on, for POSTing stats")
	nodeTier      = flag.String("nodeTier", "", "node tier the image is being pulled on, for POSTing stats")
)

func main() {
	flag.Parse()
	if len(*image) == 0 {
		log.Printf("missing image")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if !checkDocker() {
		log.Fatalf("docker not found in path.")
	}

	if !checkSkopeo() {
		log.Fatalf("skopeo not found in path.")
	}

	log.Printf("Pulling image: %s", *image)

	progressCh := make(chan int, 0)
	go func() {
		lastStat := 0
		start := time.Now()

		for p := range progressCh {
			lastStat = p
			elapsed := time.Now().Sub(start)
			if elapsed > statsInterval {
				log.Printf("Pull progress for %s: %d%%", *image, p)
				if len(*statsEndpoint) > 0 {
					if err := publishStats(*statsEndpoint, *image, lastStat, *jobName, *nodeName, *nodeTier); err != nil {
						log.Printf("ERROR: could not publish stats: %v", err)
					}
				}
				start = time.Now()
			}
			if p == 100 {
				break
			}
		}
	}()

	pullStart := time.Now()
	if err := dockerPullWithProgress(*image, progressCh, timeout); err != nil {
		log.Fatal(err)
	}

	totalPullTime := time.Now().Sub(pullStart)

	log.Printf("Done, image pulled in %.3f minutes", totalPullTime.Minutes())
}

func checkDocker() bool {
	cmd := exec.Command("docker", "version")
	_, err := cmd.CombinedOutput()
	return err == nil
}

func checkSkopeo() bool {
	cmd := exec.Command("skopeo", "-v")
	_, err := cmd.CombinedOutput()
	return err == nil
}

func publishStats(statsEndpoint, image string, progress int, jobName, nodeName, nodeTier string) error {
	stat := statSpec{
		Metadta: statMetadataSpec{
			Name: jobName,
			Node: nodeName,
			Tier: nodeTier,
		},
		Stats: statProgressSpec{
			Image:        image,
			PullProgress: progress,
		},
	}
	jsonStr, _ := json.MarshalIndent(stat, "", "  ")
	req, err := http.NewRequest("POST", statsEndpoint, bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to post stats: %s", string(body))
	}
	return nil
}
