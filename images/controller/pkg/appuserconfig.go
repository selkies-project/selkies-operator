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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

func (userConfig *AppUserConfigObject) WriteJSON(destFile string) error {
	if err := os.MkdirAll(filepath.Dir(destFile), os.ModePerm); err != nil {
		return err
	}

	data, err := json.MarshalIndent(userConfig, "", " ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(destFile, data, 0644)
}

func NewAppUserConfig(name, namespace string, spec AppUserConfigSpec) AppUserConfigObject {
	return AppUserConfigObject{
		KubeObjectBase: KubeObjectBase{
			ApiVersion: ApiVersion,
			Kind:       BrokerAppUserConfigKind,
		},
		Metadata: KubeObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       spec.AppName,
				"app.kubernetes.io/instance":   name,
				"app.kubernetes.io/managed-by": "pod-broker",
			},
			Annotations: map[string]string{},
		},
		Spec: spec,
	}
}

func NewAppUserConfigFromJSON(srcFile string) (AppUserConfigObject, error) {
	userConfig := AppUserConfigObject{}

	data, err := ioutil.ReadFile(srcFile)
	if err != nil {
		return userConfig, err
	}

	err = yaml.Unmarshal([]byte(data), &userConfig)
	return userConfig, err
}

func FetchAppUserConfigs() ([]AppUserConfigObject, error) {
	userConfigs := make([]AppUserConfigObject, 0)

	type appUserConfigItems struct {
		Items []AppUserConfigObject `json:"items"`
	}

	// Fetch all app user config objects
	// 	 kubectl get brokerappuserconfigs -l app.kubernetes.io/managed-by=pod-broker
	cmd := exec.Command("sh", "-c", "kubectl get brokerappuserconfigs --all-namespaces -l app.kubernetes.io/managed-by=pod-broker -o json")
	output, err := cmd.Output()
	if err != nil {
		return userConfigs, err
	}

	var items appUserConfigItems
	if err := json.Unmarshal(output, &items); err != nil {
		return userConfigs, err
	}
	userConfigs = items.Items

	return userConfigs, nil
}

func GetAppUserConfig(srcFile string) (AppUserConfigObject, error) {
	userConfig := AppUserConfigObject{}

	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		return userConfig, fmt.Errorf("app user config not found: %s", srcFile)
	}

	data, err := ioutil.ReadFile(srcFile)
	if err != nil {
		return userConfig, err
	}

	err = json.Unmarshal([]byte(data), &userConfig)
	if err != nil {
		return userConfig, nil
	}

	return userConfig, nil
}
