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
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

type rtcConfigResponse struct {
	LifetimeDuration   string              `json:"lifetimeDuration"`
	IceServers         []iceServerResponse `json:"iceServers"`
	BlockStatus        string              `json:"blockStatus"`
	IceTransportPolicy string              `json:"iceTransportPolicy"`
}

type iceServerResponse struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

func main() {
	discoveryDNSName := popVarFromEnv("DISCOVERY_DNS_NAME", true, "")
	listenPort := popVarFromEnv("PORT", false, "8080")
	authHeaderName := popVarFromEnv("AUTH_HEADER_NAME", false, "x-auth-user")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		// Get user from auth header.
		user := r.Header.Get(authHeaderName)
		if len(user) == 0 {
			writeStatusResponse(w, http.StatusUnauthorized, fmt.Sprintf("Missing or invalid %s header", authHeaderName))
			return
		}
		// IAP uses a prefix of accounts.google.com:email, remove this to just get the email
		userToks := strings.Split(user, ":")
		user = userToks[len(userToks)-1]

		// Fetch all service host and ports using SRV record of headless discovery service.
		_, srvs, err := net.LookupSRV("rest", "tcp", discoveryDNSName)
		if err != nil {
			writeStatusResponse(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		numServers := len(srvs)

		// Fetch all addresses in parallel.
		stunServerQueue := make(chan string, numServers)
		turnServerQueue := make(chan string, numServers)
		credentialsQueue := make(chan iceServerResponse, numServers)

		var wg sync.WaitGroup
		for _, srv := range srvs {
			wg.Add(1)
			url := fmt.Sprintf("http://%v:%v", srv.Target, srv.Port)
			go func(url string) {
				// Decrement the counter when the goroutine completes.
				defer wg.Done()

				// Fetch the URL.
				client := &http.Client{}
				req, _ := http.NewRequest("GET", url, nil)
				req.Header.Set(authHeaderName, user)
				resp, err := client.Do(req)
				if err != nil {
					log.Printf("ERROR: failed to fetch rtc config from %v: %v", url, err)
					return
				}
				defer resp.Body.Close()

				// Decode rtc config
				var rtcConfig rtcConfigResponse
				err = json.NewDecoder(resp.Body).Decode(&rtcConfig)
				if err != nil {
					log.Printf("ERROR: failed to decode JSON RTC Config from %v: %v", url, err)
					return
				}

				// Extract servers
				for _, iceServer := range rtcConfig.IceServers {
					for _, url := range iceServer.URLs {
						if url[0:4] == "stun" {
							stunServerQueue <- url
						} else if url[0:4] == "turn" {
							turnServerQueue <- url
							credentialsQueue <- iceServerResponse{
								Username:   iceServer.Username,
								Credential: iceServer.Credential,
							}
						}
					}
				}
			}(url)
		}
		wg.Wait()

		stunServers := make([]string, 0)
		turnServers := make([]string, 0)

		for i := 0; i < numServers; i++ {
			stunServers = append(stunServers, <-stunServerQueue)
			turnServers = append(turnServers, <-turnServerQueue)
		}
		close(stunServerQueue)
		close(turnServerQueue)

		// Use the first credential, all TURN services use the same shared key.
		credential := <-credentialsQueue
		close(credentialsQueue)
		turnUsername := credential.Username
		turnCredential := credential.Credential

		// Make sure we have at least 1 server.
		if len(stunServers) == 0 || len(turnServers) == 0 {
			writeStatusResponse(w, http.StatusInternalServerError, "Failed to fetch STUN/TURN servers")
			return
		}

		resp, err := makeCombinedRTCConfig(stunServers, turnServers, turnUsername, turnCredential)
		if err != nil {
			writeStatusResponse(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	})

	log.Println(fmt.Sprintf("Listening on port %s", listenPort))
	http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", listenPort), nil)
}

func makeCombinedRTCConfig(stunServers, turnServers []string, username, credential string) (rtcConfigResponse, error) {
	var resp rtcConfigResponse
	var err error

	resp.LifetimeDuration = "86400s"
	resp.BlockStatus = "NOT_BLOCKED"
	resp.IceTransportPolicy = "all"
	resp.IceServers = []iceServerResponse{
		iceServerResponse{
			URLs: stunServers,
		},
		iceServerResponse{
			URLs:       turnServers,
			Username:   username,
			Credential: credential,
		},
	}

	return resp, err
}

func writeStatusResponse(w http.ResponseWriter, statusCode int, message string) {
	type statusResponse struct {
		Status string
	}
	status := statusResponse{
		Status: message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(status)
}

func popVarFromEnv(envName string, isRequired bool, defaultValue string) string {
	value := os.Getenv(envName)
	if isRequired && len(value) == 0 {
		log.Fatalf("Missing environment variable: %s", envName)
	} else if len(value) == 0 {
		value = defaultValue
	}
	return value
}

func chanToSliceString(c chan string) []string {
	s := make([]string, 0)
	for i := range c {
		s = append(s, i)
	}
	return s
}
