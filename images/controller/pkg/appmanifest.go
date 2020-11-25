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

package pod_broker

import (
	"encoding/json"
	"io/ioutil"
)

func (manifest *RegisteredAppsManifest) Add(app AppConfigSpec) {
	manifest.Apps[app.Name] = app
}

func (manifest *RegisteredAppsManifest) WriteJSON(destFile string) error {
	data, err := json.MarshalIndent(manifest, "", " ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(destFile, data, 0644)
}

func NewRegisteredAppManifest() RegisteredAppsManifest {
	return RegisteredAppsManifest{
		Apps: make(map[string]AppConfigSpec, 0),
	}
}

func NewRegisteredAppManifestFromJSON(srcFile string, appType AppType) (RegisteredAppsManifest, error) {
	var manifest RegisteredAppsManifest

	data, err := ioutil.ReadFile(srcFile)
	if err != nil {
		return manifest, err
	}

	err = json.Unmarshal(data, &manifest)

	apps := make(map[string]AppConfigSpec, 0)
	for k, v := range manifest.Apps {
		if v.Type == appType || appType == AppTypeAll {
			apps[k] = v
		}
	}
	manifest.Apps = apps

	return manifest, err
}
