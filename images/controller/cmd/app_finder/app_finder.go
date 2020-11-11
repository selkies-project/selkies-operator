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
	"bufio"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
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
		nsConfigMaps, err := broker.GetConfigMaps(namespace)
		if err != nil {
			log.Printf("failed to fetch broker appconfig map bundles: %v", err)
		}

		// Fetch data required for Egress Network Policy
		networkPolicyData, err := broker.GetEgressNetworkPolicyData(namespace)
		if err != nil {
			log.Printf("failed to fetch networkpolicy data: %v", err)
		}
		registeredApps.NetworkPolicyData = networkPolicyData

		// Temp directory where updated files are staged.
		tmpDir := path.Dir(broker.BundleSourceBaseDir)

		for _, appConfig := range appConfigs {
			bundleCMName := appConfig.Spec.Bundle.ConfigMapRef.Name
			userBundleCMName := appConfig.Spec.UserBundle.ConfigMapRef.Name
			authzCMName := appConfig.Spec.Authorization.ConfigMapRef.Name

			appName := appConfig.Metadata.Name
			bundleDestDir := path.Join(broker.BundleSourceBaseDir, appName)
			userBundleDestDir := path.Join(broker.UserBundleSourceBaseDir, appName)

			// Find and save configmap data for required bundle and optional authz
			foundBundle := false
			foundUserBundle := false
			foundAuthzCM := false
			for _, cm := range nsConfigMaps {
				// Match on bundle configmap name
				if cm.Metadata.Name == bundleCMName {
					if err := cm.SaveDataToDirectory(bundleDestDir, tmpDir); err != nil {
						log.Printf("failed to save bundle ConfigMap '%s' to %s: %v", cm.Metadata.Name, bundleDestDir, err)
					} else {
						foundBundle = true
					}
				}

				// Match on user bundle configmap name
				if cm.Metadata.Name == userBundleCMName {
					if err := cm.SaveDataToDirectory(userBundleDestDir, tmpDir); err != nil {
						log.Printf("failed to save user bundle ConfigMap '%s' to %s: %v", cm.Metadata.Name, userBundleDestDir, err)
					} else {
						foundUserBundle = true
					}
				}

				// Match on authz configmap name
				if cm.Metadata.Name == authzCMName {
					foundAuthzCM = true
					// Extract authorization members and append them to the appConfig.Spec.AuthorizedUsers array.
					for _, data := range cm.Data {
						scanner := bufio.NewScanner(strings.NewReader(data))
						for scanner.Scan() {
							userPat := scanner.Text()
							// Skip comment lines.
							if !strings.HasPrefix(userPat, "#") {
								_, err := regexp.Compile(userPat)
								if err != nil {
									log.Printf("WARN: invalid authorized user pattern found in ConfigMap %s: '%s', skipped.", authzCMName, userPat)
								} else {
									appConfig.Spec.AuthorizedUsers = append(appConfig.Spec.AuthorizedUsers, userPat)
								}
							}
						}
					}
				}
			}

			if !foundBundle {
				log.Printf("Bundle manifests ConfigMap %s not found for app %s", bundleCMName, appName)
			} else if len(userBundleCMName) > 0 && !foundUserBundle {
				log.Printf("User bundle manifests ConfigMap %s not found for app %s", userBundleCMName, appName)
			} else {
				if len(authzCMName) > 0 && !foundAuthzCM {
					log.Printf("Failed to find authorization ConfigMap bundle %s for app %s", authzCMName, appName)
				}
				// App is valid and bundle is ready, add to registered apps.
				if !appConfig.Spec.Disabled {
					registeredApps.Add(appConfig.Spec)
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
