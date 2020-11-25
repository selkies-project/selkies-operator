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

	// Set default values
	for i := range items.Items {
		// Default app type to StatefulSet
		if items.Items[i].Spec.Type == "" {
			items.Items[i].Spec.Type = AppTypeStatefulSet
		}

		// Default userBundles to empty list if not provided.
		if items.Items[i].Spec.UserBundles == nil {
			items.Items[i].Spec.UserBundles = make([]UserBundleSpec, 0)
		}

		if items.Items[i].Spec.Type == AppTypeDeployment {
			// Default deployment selector to match app name.
			if len(items.Items[i].Spec.Deployment.Selector) == 0 {
				items.Items[i].Spec.Deployment.Selector = fmt.Sprintf("app=%s", items.Items[i].Spec.Name)
			}

			if items.Items[i].Spec.Deployment.Replicas == nil {
				// Default number of deployment replicas.
				// This value is a pointer so that it can accept 0 as a valid value.
				defaultReplicas := DefaultDeploymentReplicas
				items.Items[i].Spec.Deployment.Replicas = &defaultReplicas
			}
		}
	}

	return items.Items, err
}
