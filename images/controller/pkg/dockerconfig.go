/*
 Copyright 2021 The Selkies Authors. All rights reserved.

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
	"log"
	"strings"
	"sync"

	registry_authn "github.com/google/go-containerregistry/pkg/authn"
	registry_name "github.com/google/go-containerregistry/pkg/name"
)

type DockerConfigJSON struct {
	// Map of registry URL to credential
	Auths map[string]registry_authn.AuthConfig `json:"auths"`
}

type DockerConfigsSync struct {
	sync.Mutex
	Configs []DockerConfigJSON
	Secrets []string
}

func (dc *DockerConfigsSync) GetAuthConfigsForRepo(image string) ([]registry_authn.AuthConfig, error) {
	resp := []registry_authn.AuthConfig{}

	dc.Lock()
	defer dc.Unlock()
	nameRef, err := registry_name.ParseReference(image)
	if err != nil {
		return resp, err
	}

	repoRegistry := nameRef.Context().RegistryStr()
	for _, dockerConfig := range dc.Configs {
		for registryURL, authConfig := range dockerConfig.Auths {
			if strings.Contains(registryURL, repoRegistry) {
				resp = append(resp, authConfig)
			}
		}
	}
	return resp, nil
}

func (dc *DockerConfigsSync) Update(namespace string) error {
	dockerConfigs, secrets, err := GetDockerConfigs(namespace)
	if err != nil {
		return err
	}

	numSecretsPrev := len(dc.Secrets)

	defaultSAConfig, err := GetDefaultSADockerConfig()
	if err != nil {
		return err
	}

	dc.Lock()
	defer dc.Unlock()
	dc.Configs = dockerConfigs
	dc.Configs = append(dc.Configs, defaultSAConfig)
	dc.Secrets = secrets

	if len(dc.Secrets) != numSecretsPrev {
		log.Printf("found %d image pull secrets:", len(dc.Secrets))
		for _, secretName := range dc.Secrets {
			log.Printf("  %s", secretName)
		}
	}

	return nil
}

func (dc *DockerConfigsSync) ListTags(repo string) ([]string, error) {
	authConfigs, _ := dc.GetAuthConfigsForRepo(repo)
	return DockerRemoteRegistryListTags(repo, authConfigs)
}

func (dc *DockerConfigsSync) GetDigest(image string) (string, error) {
	resp := ""
	repo, err := GetDockerRepoFromImage(image)
	if err != nil {
		return resp, err
	}
	authConfigs, err := dc.GetAuthConfigsForRepo(repo)
	if err != nil {
		return resp, err
	}
	return DockerRemoteRegistryGetDigest(image, authConfigs)
}

func (dc *DockerConfigsSync) GetDockerConfigJSONForRepo(image string) (string, error) {
	resp := ""

	registry, err := GetDockerImageRegistryURI(image)
	if err != nil {
		return resp, err
	}

	authConfigs, err := dc.GetAuthConfigsForRepo(image)
	if err != nil {
		return resp, err
	}
	authConfig, err := GetDockerAuthConfigForRepo(image, authConfigs)
	if err != nil {
		return resp, err
	}
	dockerConfigJSON := DockerConfigJSON{
		Auths: map[string]registry_authn.AuthConfig{
			registry: authConfig,
		},
	}
	dockerConfigJSONData, err := json.Marshal(&dockerConfigJSON)
	if err != nil {
		return resp, err
	}
	resp = string(dockerConfigJSONData)
	return resp, nil
}
