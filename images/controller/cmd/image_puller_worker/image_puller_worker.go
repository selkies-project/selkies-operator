package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	timeout       = 30 * time.Minute
	statsInterval = 1 * time.Second
)

var (
	image     = flag.String("image", "", "image name and tag to pull")
	statsPort = flag.Int("stats-port", 9100, "port to serve promethus stats on")
	exitDelay = flag.Int("exit-delay", 0, "seconds to delay exit by, to add more time to fetch stats")
)

var (
	metricPullProgress = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "image_pull_progress_percent",
		Help: "The percentage of the image pull progress",
	})
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

	// Start metrics server.
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Printf("starting prometheus server at :%d/metrics", *statsPort)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *statsPort), nil))
	}()

	log.Printf("Pulling image: %s", *image)

	progressCh := make(chan int, 0)
	pullStart := time.Now()
	go func() {
		if err := dockerPullWithProgress(*image, progressCh, timeout); err != nil {
			log.Fatal(err)
		}
	}()

	printTime := time.Now()
	for p := range progressCh {
		metricPullProgress.Set(float64(p))

		now := time.Now()
		if now.Sub(printTime) > 1*time.Second {
			log.Printf("Pull progress for %s: %d%%", *image, p)
			printTime = now
		}
	}

	totalPullTime := time.Now().Sub(pullStart)

	log.Printf("Done, image pulled in %.3f minutes", totalPullTime.Minutes())

	if *exitDelay > 0 {
		time.Sleep(time.Duration(*exitDelay) * time.Second)
	}
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
