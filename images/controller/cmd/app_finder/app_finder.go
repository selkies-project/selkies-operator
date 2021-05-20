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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	broker "selkies.io/controller/pkg"
)

func main() {

	log.Printf("Starting broker app discovery service")

	// Set from downward API.
	namespace := os.Getenv("NAMESPACE")
	if len(namespace) == 0 {
		log.Fatal("Missing NAMESPACE env.")
	}

	// Allow for single run
	singleIteration := false
	if os.Getenv("SINGLE_ITERATION") == "true" {
		singleIteration = true
	}

	// Map of cached app manifest checksums
	bundleManifestChecksums := make(map[string]string, 0)
	userBundleManifestChecksums := make(map[string]string, 0)

	for {
		// Fetch all broker apps
		appConfigs, err := broker.FetchBrokerAppConfigs(namespace)
		if err != nil {
			log.Printf("failed to fetch broker app configs: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// List of app specs that will be added to the registered app manifest.
		registeredApps := broker.NewRegisteredAppManifest()

		// Fetch all configmaps in single call
		nsConfigMaps, err := broker.GetConfigMaps(namespace)
		if err != nil {
			log.Printf("failed to fetch broker appconfig map bundles: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Fetch data required for Egress Network Policy
		networkPolicyData, err := broker.GetEgressNetworkPolicyData(namespace)
		if err != nil {
			log.Printf("failed to fetch networkpolicy data: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		registeredApps.NetworkPolicyData = networkPolicyData

		// Base dir for temp directory where updated files are staged.
		tmpDirBase := path.Dir(broker.BundleSourceBaseDir)

		for _, appConfig := range appConfigs {
			bundleCMName := appConfig.Spec.Bundle.ConfigMapRef.Name
			authzCMName := appConfig.Spec.Authorization.ConfigMapRef.Name

			appName := appConfig.Metadata.Name
			bundleDestDir := path.Join(broker.BundleSourceBaseDir, appName)
			userBundleDestDir := path.Join(broker.UserBundleSourceBaseDir, appName)
			foundUserBundleCount := 0

			// Find and save configmap data for required bundle and optional authz
			foundBundle := false
			foundAllUserBundles := false
			foundAuthzCM := false
			for _, cm := range nsConfigMaps {
				// Match on bundle configmap name
				if cm.Metadata.Name == bundleCMName {
					// Update working mainfests if bundle has changed.
					cacheKey := fmt.Sprintf("%s-%s", appName, cm.Metadata.Name)
					if err := copyConfigMapDataIfChanged(cm, tmpDirBase, bundleDestDir, cacheKey, bundleManifestChecksums); err != nil {
						log.Printf("%v", err)
					} else {
						foundBundle = true
					}
				}

				// Match on a user bundle configmap name
				for i, userBundle := range appConfig.Spec.UserBundles {
					userBundleCMName := userBundle.ConfigMapRef.Name

					// Match on user bundle configmap name
					if cm.Metadata.Name == userBundleCMName {
						destDir := path.Join(userBundleDestDir, fmt.Sprintf("%d", i))
						cacheKey := fmt.Sprintf("%s-%s", appName, cm.Metadata.Name)
						if err := copyConfigMapDataIfChanged(cm, tmpDirBase, destDir, cacheKey, userBundleManifestChecksums); err != nil {
							log.Printf("%v", err)
						} else {
							foundUserBundleCount++
						}
					}
				}

				// Match on authz configmap name
				if cm.Metadata.Name == authzCMName {
					foundAuthzCM = true
					// Extract authorization members and append them to the appConfig.Spec.AuthorizedUsers array.
					for _, data := range cm.Data {
						scanner := bufio.NewScanner(strings.NewReader(data))
						for scanner.Scan() {
							userPat := strings.TrimSpace(scanner.Text())
							// Skip comment and empty lines.
							if len(userPat) > 0 && !strings.HasPrefix(userPat, "#") {
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

			if foundUserBundleCount == len(appConfig.Spec.UserBundles) {
				foundAllUserBundles = true
			}

			if !foundBundle {
				log.Printf("Bundle manifests ConfigMap %s not found for app %s", bundleCMName, appName)
			} else if len(appConfig.Spec.UserBundles) > 0 && !foundAllUserBundles {
				log.Printf("Failed to find all spec.userBundles for app %s", appName)
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
				log.Printf("removing build source: %s", dirName)
				os.RemoveAll(dirName)
			}
		}

		// Write registered app manifest
		if err := registeredApps.WriteJSON(broker.RegisteredAppsManifestJSONFile); err != nil {
			log.Printf("failed to write registered app manifest: %s: %v", broker.RegisteredAppsManifestJSONFile, err)
		}

		if singleIteration {
			break
		}
		time.Sleep(5 * time.Second)
	}
}

// Helper function to only update the working manifest if the configmap content has changed.
func copyConfigMapDataIfChanged(cm broker.ConfigMapObject, tmpDirBase, destDir, cacheKey string, checksums map[string]string) error {
	tmpDir, err := ioutil.TempDir(tmpDirBase, "bundle")
	if err != nil {
		return fmt.Errorf("failed to create staging dir at base: %s: %v", tmpDirBase, err)
	}
	defer os.RemoveAll(tmpDir)

	if err := cm.SaveDataToDirectory(tmpDir); err != nil {
		return fmt.Errorf("failed to save bundle ConfigMap '%s' to %s: %v", cm.Metadata.Name, tmpDir, err)
	}

	// Compute and cache checksum to know if we need to update the working mainfests.
	prevChecksum := checksums[cacheKey]
	if checksums[cacheKey], err = broker.ChecksumDeploy(tmpDir); err != nil {
		return fmt.Errorf("failed to checksum build output directory: %v", err)
	}
	if prevChecksum != checksums[cacheKey] {
		log.Printf("%s manifest checksum: %s", cacheKey, checksums[cacheKey])
		if err := os.MkdirAll(path.Dir(destDir), os.ModePerm); err != nil {
			return err
		}
		os.RemoveAll(destDir)
		if err := os.Rename(tmpDir, destDir); err != nil {
			return fmt.Errorf("failed to move manifest bundle ConfigMap data to %s: %v", destDir, err)
		}
	}

	return nil
}
