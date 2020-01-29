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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"gopkg.in/yaml.v2"
)

func (cm *ConfigMapObject) SaveDataToDirectory(destDir string) error {
	// Copy the data to a temp directory, then move it to the destination.
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return err
	}

	tmpDir, err := ioutil.TempDir(BundleSourceBaseDir, "bundle")
	if err != nil {
		return err
	}
	// Ensure temp dir is cleaned up
	defer os.RemoveAll(tmpDir)

	for fileName, data := range cm.Data {
		destFile := path.Join(tmpDir, fileName)
		if err := ioutil.WriteFile(destFile, []byte(data), 0644); err != nil {
			return err
		}
	}

	os.RemoveAll(destDir)
	return os.Rename(tmpDir, destDir)
}

func GetConfigMaps(namespace string) ([]ConfigMapObject, error) {
	objs := make([]ConfigMapObject, 0)

	type configMapItems struct {
		Items []ConfigMapObject `yaml:"items,omitempty"`
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get configmaps -n %s -o yaml", namespace))
	output, err := cmd.Output()
	if err != nil {
		return objs, err
	}

	var items configMapItems
	err = yaml.Unmarshal(output, &items)
	if err != nil {
		return objs, err
	}
	if items.Items == nil {
		// Possible single-value query.
		var obj ConfigMapObject
		err = yaml.Unmarshal(output, &obj)
		if err != nil {
			return objs, err
		}
		objs = append(objs, obj)
	} else {
		for _, item := range items.Items {
			objs = append(objs, item)
		}
	}
	return objs, err
}
