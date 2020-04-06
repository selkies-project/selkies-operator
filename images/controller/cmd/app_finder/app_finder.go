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
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	broker "gcp.solutions/kube-app-launcher/pkg"
)

func main() {

	log.Printf("Starting broker app discovery service")

	// Set from downward API.
	namespace := os.Getenv("NAMESPACE")
	if len(namespace) == 0 {
		log.Fatal("Missing NAMESPACE env.")
	}

	for {
		// Fetch all user app configs
		appConfigs, err := broker.FetchBrokerAppConfigs(namespace)
		if err != nil {
			log.Printf("failed to fetch broker app configs: %v", err)
		}

		// List of app specs that will be added to the registered app manifest.
		registeredApps := broker.NewRegisteredAppManifest()

		// Fetch all configmaps in single call
		bundleConfigMaps, err := broker.GetConfigMaps(namespace)
		if err != nil {
			log.Printf("failed to fetch broker appconfig map bundles: %v", err)
		}

		// Fetch data required for Egress Network Policy
		networkPolicyData, err := broker.GetEgressNetworkPolicyData(namespace, "app.kubernetes.io/name=broker")
		if err != nil {
			log.Printf("failed to fetch networkpolicy data: %v", err)
		}
		registeredApps.NetworkPolicyData = networkPolicyData

		for _, appConfig := range appConfigs {
			cmName := appConfig.Spec.Bundle.ConfigMapRef.Name
			appName := appConfig.Metadata.Name

			var cm broker.ConfigMapObject
			found := false
			for _, cm = range bundleConfigMaps {
				if cm.Metadata.Name == cmName {
					found = true
					break
				}
			}
			if !found {
				log.Printf("ConfigMap %s not found for app %s", cmName, appName)
			} else {
				// Save ConfigMap data to shared build source directory.
				destDir := path.Join(broker.BundleSourceBaseDir, appName)
				if err := cm.SaveDataToDirectory(destDir); err != nil {
					log.Printf("failed to save ConfigMap bundle '%s' to %s: %v", cm.Metadata.Name, destDir, err)
				} else {
					// App is valid and bundle is ready, add to registered apps.
					if !appConfig.Spec.Disabled {
						registeredApps.Add(appConfig.Spec)
					}
				}
			}
		}

		// Prune build source directories
		foundDirs, err := filepath.Glob(path.Join(broker.BundleSourceBaseDir, "*"))
		if err != nil {
			log.Printf("failed to list app directories to prune: %v", err)
		}
		for _, dirName := range foundDirs {
			found := false
			for _, appConfig := range appConfigs {
				if appConfig.Metadata.Name == path.Base(dirName) {
					found = true
					break
				}
			}
			if !found {
				os.RemoveAll(dirName)
			}
		}

		// Write registered app manifest
		if err := registeredApps.WriteJSON(broker.RegisteredAppsManifestJSONFile); err != nil {
			log.Printf("failed to write registered app manifest: %s: %v", broker.RegisteredAppsManifestJSONFile, err)
		}

		time.Sleep(5 * time.Second)
	}
}
