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
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
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
	externalIP := popVarFromEnv("EXTERNAL_IP", true, "")
	turnPort := popVarFromEnv("TURN_PORT", false, "3478")
	sharedSecret := popVarFromEnv("TURN_SHARED_SECRET", true, "")
	listenPort := popVarFromEnv("PORT", false, "8080")
	authHeaderName := strings.ToLower(popVarFromEnv("AUTH_HEADER_NAME", false, "x-auth-user"))

	// Env vars for running in aggregator mode.
	discoveryDNSName := popVarFromEnv("DISCOVERY_DNS_NAME", false, "")
	discoveryPortName := popVarFromEnv("DISCOVERY_PORT_NAME", false, "turn")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Get user from auth header.
		authHeaderValue := r.Header.Get(authHeaderName)
		user := ""
		if authHeaderName == "x-goog-authenticated-user-email" {
			// IAP uses a prefix of accounts.google.com:email, remove this to just get the email
			userToks := strings.Split(authHeaderValue, ":")
			if len(userToks) > 1 {
				user = userToks[len(userToks)-1]
			}
		} else {
			user = authHeaderValue
		}
		if len(user) == 0 {
			writeStatusResponse(w, http.StatusUnauthorized, fmt.Sprintf("Failed to get user from auth header: '%s'", authHeaderName))
			return
		}

		ips := make([]string, 0)

		if len(discoveryDNSName) == 0 {
			// Standard mode, use external IP to return single server.
			ips = []string{externalIP}
			resp, err := makeRTCConfig(ips, turnPort, user, sharedSecret)
			if err != nil {
				writeStatusResponse(w, http.StatusInternalServerError, "Internal server error")
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		} else {
			// Aggregator mode, use DNS SRV records to build RTC config.
			// Fetch all service host and ports using SRV record of headless discovery service.
			// NOTE: The SRV record returns resolvable aliases to the endpoints, so do another lookup should return the IP.
			_, srvs, err := net.LookupSRV(discoveryPortName, "tcp", discoveryDNSName)
			if err != nil {
				log.Printf("ERROR: failed to query SRV record from %v %v: %v", discoveryDNSName, discoveryPortName, err)
				writeStatusResponse(w, http.StatusInternalServerError, "Internal server error")
				return
			}
			for _, srv := range srvs {
				addrs, err := net.LookupHost(srv.Target)
				if err != nil {
					log.Printf("ERROR: failed to query A record from %v: %v", srv.Target, err)
					writeStatusResponse(w, http.StatusInternalServerError, "Internal server error")
					return
				}
				ips = append(ips, addrs[0])
			}
		}

		resp, err := makeRTCConfig(ips, turnPort, user, sharedSecret)
		if err != nil {
			log.Printf("ERROR: failed to make RTC config: %v", err)
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

func makeRTCConfig(ips []string, port, user, secret string) (rtcConfigResponse, error) {
	var resp rtcConfigResponse
	var err error

	username, credential := makeCredential(secret, user)

	stunURLs := []string{}
	turnURLs := []string{}

	for _, ip := range ips {
		stunURLs = append(stunURLs, fmt.Sprintf("stun:%s:%s", ip, port))
		turnURLs = append(turnURLs, fmt.Sprintf("turn:%s:%s?transport=udp", ip, port))
	}

	resp.LifetimeDuration = "86400s"
	resp.BlockStatus = "NOT_BLOCKED"
	resp.IceTransportPolicy = "all"
	resp.IceServers = []iceServerResponse{
		iceServerResponse{
			URLs: stunURLs,
		},
		iceServerResponse{
			URLs:       turnURLs,
			Username:   username,
			Credential: credential,
		},
	}

	return resp, err
}

// Creates credential per coturn REST API docs
// https://github.com/coturn/coturn/wiki/turnserver#turn-rest-api
// [START makeCredential]
func makeCredential(secret, user string) (string, string) {
	var username string
	var credential string

	ttlOneDay := 24 * 3600 * time.Second
	nowPlusTTL := time.Now().Add(ttlOneDay).Unix()
	// Make sure to set --rest-api-separator="-" in the coturn config.
	username = fmt.Sprintf("%d-%s", nowPlusTTL, user)

	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(username))
	credential = base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return username, credential
}

// [END makeCredential]

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
