package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	expect "github.com/google/goexpect"
)

type platformSpec struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
}
type manifestSpec struct {
	Digest    string        `json:"digest"`
	MediaType string        `json:"mediaType"`
	Platform  *platformSpec `json:"platform"`
	Size      int64         `json:"size"`
}
type layerSpec struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}

type inspectSpec struct {
	SchemaVersion int            `json:"schemaVersion"`
	MediaType     string         `json:"mediaType"`
	Manifests     []manifestSpec `json:"manifests"`
	Layers        []layerSpec    `json:"layers"`
}

var (
	startRE       = regexp.MustCompile(`Pulling from (.*)`)
	downloadingRE = regexp.MustCompile(`\s+([a-f0-9]{12}): Downloading.*?([0-9.]+)(.*?)/([0-9.]+)(.*?)[\n\r]+?`)
	extractingRE  = regexp.MustCompile(`\s+([a-f0-9]{12}): Extracting.*?([0-9.]+)(.*?)/([0-9.]+)(.*?)[\n\r]+?`)
	completeRE    = regexp.MustCompile(`Status: (Downloaded newer image|Image is up to date)`)
)

func dockerPullWithProgress(image string, progressCh chan<- int, timeout time.Duration) error {
	var match []string
	var err error
	var count int
	var totalPercent int = 0
	layerDownloadStatus := make(map[string]int, 0)
	layerExtractStatus := make(map[string]int, 0)
	layerWeights := make(map[string]float64, 0)

	defer close(progressCh)

	// Prepopulate status maps, initialize all progress percentages to 0.
	digest, layers, err := getImageLayers(image)
	if err != nil {
		return err
	}
	log.Printf("%s, digest: %s, size: %d", image, digest, layerSizeSum(layers))

	var totalSize int64 = 0
	for _, l := range layers {
		shortHash := l.Digest[7:19]
		layerDownloadStatus[shortHash] = 0
		layerExtractStatus[shortHash] = 0
		totalSize += l.Size
		log.Printf("layer: %s: %d", l.Digest, l.Size)
	}

	// Compute layer weights proportional to the size of layers with respect to the total image size.
	for _, l := range layers {
		shortHash := l.Digest[7:19]
		layerWeights[shortHash] = float64(l.Size) / float64(totalSize)
	}

	e, _, err := expect.Spawn(fmt.Sprintf("docker pull %s", image), -1, expect.CheckDuration(50*time.Millisecond))
	if err != nil {
		log.Fatal(err)
	}
	defer e.Close()

	for {
		_, match, count, err = e.ExpectSwitchCase([]expect.Caser{
			&expect.Case{R: startRE, T: expect.OK()},
			&expect.Case{R: downloadingRE, T: expect.OK()},
			&expect.Case{R: extractingRE, T: expect.OK()},
			&expect.Case{R: completeRE, T: expect.OK()},
		}, timeout)
		if count < 0 || err != nil {
			break
		}
		switch count {
		case 0:
			// Pull started
			totalPercent = 0
		case 1:
			// Layer download progress

			shortHash := match[1]
			curr := match[2]
			currUnit := match[3]
			total := match[4]
			totalUnit := match[5]

			percent, err := computeLayerPercent(curr, currUnit, total, totalUnit)
			if err != nil {
				log.Printf("ERROR: %v", err)
			}

			//log.Printf("DOWNLOADING Layer: %s: %d", shortHash, percent)
			layerDownloadStatus[shortHash] = percent
			totalPercent = computeTotalPercent(layerDownloadStatus, layerExtractStatus, layerWeights)
		case 2:
			// Layer extract progress

			shortHash := match[1]
			curr := match[2]
			currUnit := match[3]
			total := match[4]
			totalUnit := match[5]

			percent, err := computeLayerPercent(curr, currUnit, total, totalUnit)
			if err != nil {
				log.Printf("ERROR: %v", err)
			}

			layerExtractStatus[shortHash] = percent
			totalPercent = computeTotalPercent(layerDownloadStatus, layerExtractStatus, layerWeights)
		case 3:
			// Pull complete
			totalPercent = 100
		}

		progressCh <- totalPercent
	}

	return nil
}

func unitToBytes(value float64, unit string) (int, error) {
	b := 0

	switch unit {
	case "B":
		b = int(value)
	case "kB":
		b = int(value * 1024)
	case "MB":
		b = int(value * 1024 * 1024)
	case "GB":
		b = int(value * 1024 * 1024 * 1024)
	default:
		return b, fmt.Errorf("Unsupported docker pull size unit: '%s'", unit)
	}

	return b, nil
}

func computeLayerPercent(curr, currUnit, total, totalUnit string) (int, error) {
	percent := 0

	currFloat, err := strconv.ParseFloat(curr, 64)
	if err != nil {
		return percent, err
	}

	totalFloat, err := strconv.ParseFloat(total, 64)
	if err != nil {
		return percent, err
	}

	currBytes, err := unitToBytes(currFloat, currUnit)
	if err != nil {
		return percent, err
	}

	totalBytes, err := unitToBytes(totalFloat, totalUnit)
	if err != nil {
		return percent, err
	}

	percent = int(float64(currBytes) / float64(totalBytes) * 100)

	return percent, nil
}

func computeTotalPercent(layerDownloadStatus, layerExtractStatus map[string]int, layerWeights map[string]float64) int {
	percent := 0

	downloadWeight := 0.5
	extractWeight := 0.5

	downloadPercent := 0.0
	for l, v := range layerDownloadStatus {
		downloadPercent += float64(v) * layerWeights[l]
	}

	extractPercent := 0.0
	for l, v := range layerExtractStatus {
		extractPercent += float64(v) * layerWeights[l]
	}

	percent = int(downloadPercent*downloadWeight + extractPercent*extractWeight)
	return percent
}

// Return digest of image manifest and layers
// If the mediaType is application/vnd.docker.distribution.manifest.list.v2+json,
//   then select the linux amd64 variant, if found, if not, returns error.
func getImageLayers(image string) (string, []layerSpec, error) {
	digest := ""
	layers := make([]layerSpec, 0)

	var imageRepo string

	imageTagToks := strings.Split(image, ":")
	if len(imageTagToks) == 2 {
		imageRepo = imageTagToks[0]
	} else {
		imageTagToks := strings.Split(image, "@")
		if len(imageTagToks) == 2 {
			imageRepo = imageTagToks[0]
		}
	}

	var inspectRes inspectSpec

	cmd := exec.Command("skopeo", "inspect", "--raw", fmt.Sprintf("docker://%s", image))
	manifestJSON, err := cmd.CombinedOutput()
	if err != nil {
		return digest, layers, err
	}

	json.Unmarshal(manifestJSON, &inspectRes)

	if inspectRes.MediaType == "application/vnd.docker.distribution.manifest.list.v2+json" {
		log.Printf("Need to fetch variant")
		for _, m := range inspectRes.Manifests {
			if m.Platform.OS == "linux" && m.Platform.Architecture == "amd64" {
				log.Printf("linux amd64 variant digest: %s", m.Digest)
			}
			// Fetch manifest with shasum of variant digest.
			cmd := exec.Command("skopeo", "inspect", "--raw", fmt.Sprintf("docker://%s@%s", imageRepo, m.Digest))
			stdoutStderr, err := cmd.CombinedOutput()
			if err != nil {
				return digest, layers, err
			}
			var variantInspectRes inspectSpec
			json.Unmarshal(stdoutStderr, &variantInspectRes)
			if len(variantInspectRes.Layers) == 0 {
				return digest, layers, fmt.Errorf("No layers found in manifest")
			}
			for _, layer := range variantInspectRes.Layers {
				layers = append(layers, layer)
			}
			break
		}
		if len(manifestJSON) == 0 {
			return digest, layers, fmt.Errorf("unable to find linux amd64 variant for image")
		}
	} else if len(inspectRes.Layers) == 0 {
		return digest, layers, fmt.Errorf("unsupported image manifest mediaType: %s", inspectRes.MediaType)
	} else {
		for _, layer := range inspectRes.Layers {
			layers = append(layers, layer)
		}
	}

	hash := sha256.New()
	if _, err := hash.Write(manifestJSON); err != nil {
		return digest, layers, err
	}
	sum := hash.Sum(nil)
	digest = fmt.Sprintf("sha256@%x", sum)
	return digest, layers, nil
}

func layerSizeSum(layers []layerSpec) int64 {
	var sum int64 = 0
	for _, layer := range layers {
		sum += layer.Size
	}
	return sum
}
