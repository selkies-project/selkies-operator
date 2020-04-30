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
	"os/exec"
)

func (spec *AppConfigSpec) NodeTierNames() []string {
	tierNames := []string{}
	for _, tier := range spec.NodeTiers {
		tierNames = append(tierNames, tier.Name)
	}
	return tierNames
}

func FetchBrokerAppConfigs(namespace string) ([]AppConfigObject, error) {
	appConfigs := make([]AppConfigObject, 0)

	type appConfigItems struct {
		Items []AppConfigObject `yaml:"items"`
	}

	// Fetch all broker app configs
	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl get brokerappconfigs -n %s -o json", namespace))
	output, err := cmd.Output()
	if err != nil {
		return appConfigs, err
	}

	var items appConfigItems
	err = json.Unmarshal(output, &items)
	return items.Items, err
}
