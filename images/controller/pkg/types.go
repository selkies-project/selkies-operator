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

const apiVersion = "gcp.solutions/v1"
const brokerAppConfigKind = "BrokerAppUserConfig"
const brokerAppUserConfigKind = "BrokerAppUserConfig"

const BrokerCommonBuildSouceBaseDir = "/opt/broker/buildsrc"
const BundleSourceBaseDir = "/var/run/buildsrc/apps"
const BuildSourceBaseDir = "/var/run/build"
const BuildSourceExtrasDir = "/opt/broker/buildsrc/extra"
const RegisteredAppsManifestJSONFile = "/var/run/buildsrc/apps.json"
const AppUserConfigBaseDir = "/var/run/userconfig"
const AppUserConfigJSONFile = "app-user-config.json"

type KubeObjectBase struct {
	ApiVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
}

type KubeObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Annotations map[string]string `json:"annotations"`
	Labels      map[string]string `json:"labels"`
}

type ConfigMapData map[string]string

type ConfigMapObject struct {
	KubeObjectBase
	Metadata KubeObjectMeta `yaml:"metadata" json:"metadata"`
	Data     ConfigMapData  `yaml:"data" json:"data"`
}

// Data passed to template generator
type UserPodData struct {
	Namespace                 string
	AppSpec                   AppConfigSpec
	App                       string
	AppUserConfig             AppUserConfigSpec
	ImageRepo                 string
	ImageTag                  string
	NodeTier                  NodeTierSpec
	Domain                    string
	User                      string
	CookieValue               string
	ID                        string
	FullName                  string
	ServiceName               string
	Resources                 []string
	Patches                   []string
	JSONPatchesService        []string
	JSONPatchesVirtualService []string
	JSONPatchesDeploy         []string
	UserParams                map[string]string
	AppParams                 map[string]string
	SysParams                 map[string]string
}

type NodeResource struct {
	CPU    int    `yaml:"cpu,omitempty" json:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

type NodeResourceRequestSpec struct {
	Requests NodeResource `yaml:"requests,omitempty" json:"requests,omitempty"`
	Limits   NodeResource `yaml:"limits,omitempty" json:"limits,omitempty"`
}

type NodeTierSpec struct {
	Name      string                  `yaml:"name" json:"name"`
	NodeLabel string                  `yaml:"nodeLabel" json:"nodeLabel"`
	Resources NodeResourceRequestSpec `yaml:"resources" json:"resources"`
}

type ConfigMapRef struct {
	Name string `yaml:"name" json:"name"`
}

type BundleSpec struct {
	ConfigMapRef ConfigMapRef `yaml:"configMapRef" json:"configMapRef"`
}

type AppConfigParam struct {
	Name        string `yaml:"name" json:"name"`
	DisplayName string `yaml:"displayName" json:"displayName"`
	Type        string `yaml:"type" json:"type"`
	Default     string `yaml:"default" json:"default"`
}

type AppImageSpec struct {
	Name    string `yaml:"name" json:"name"`
	OldRepo string `yaml:"oldRepo" json:"oldRepo"`
	NewRepo string `yaml:"newRepo" json:"newRepo"`
	NewTag  string `yaml:"newTag" json:"newTag"`
	Digest  string `yaml:"digest,omitempty" json:"digest,omitempty"`
}

type AppConfigSpec struct {
	Name        string                  `yaml:"name" json:"name"`
	DisplayName string                  `yaml:"displayName" json:"displayName"`
	Description string                  `yaml:"description" json:"description"`
	Icon        string                  `yaml:"icon,omitempty" json:"icon,omitempty"`
	LaunchURL   string                  `yaml:"launchURL,omitempty" json:"launchURL,omitempty"`
	Disabled    bool                    `yaml:"disabled" json:"disabled"`
	Version     string                  `yaml:"version" json:"version"`
	Bundle      BundleSpec              `yaml:"bundle" json:"bundle"`
	DefaultRepo string                  `yaml:"defaultRepo" json:"defaultRepo"`
	DefaultTag  string                  `yaml:"defaultTag" json:"defaultTag"`
	Images      map[string]AppImageSpec `yaml:"images,omitempty" json:"images,omitempty"`
	NodeTiers   []NodeTierSpec          `yaml:"nodeTiers,omitempty" json:"nodeTiers,omitempty"`
	DefaultTier string                  `yaml:"defaultTier,omitempty" json:"defaultTier,omitempty"`
	ServiceName string                  `yaml:"serviceName" json:"serviceName"`
	UserParams  []AppConfigParam        `yaml:"userParams,omitempty" json:"userParams,omitempty"`
	AppParams   []AppConfigParam        `yaml:"appParams,omitempty" json:"appParams,omitempty"`
}

type AppConfigObject struct {
	KubeObjectBase
	Metadata KubeObjectMeta `yaml:"metadata" json:"metadata"`
	Spec     AppConfigSpec  `yaml:"spec" json:"spec"`
}

type AppUserConfigSpec struct {
	AppName   string            `yaml:"appName" json:"appName"`
	User      string            `yaml:"user" json:"user"`
	ImageRepo string            `yaml:"imageRepo,omitempty" json:"imageRepo,omitempty"`
	ImageTag  string            `yaml:"imageTag,omitempty" json:"imageTag,omitempty"`
	Tags      []string          `yaml:"tags" json:"tags"`
	NodeTier  string            `yaml:"nodeTier,omitempty" json:"nodeTier,omitempty"`
	Params    map[string]string `yaml:"params" json:"params"`
}

type AppUserConfigObject struct {
	KubeObjectBase
	Metadata KubeObjectMeta    `yaml:"metadata" json:"metadata"`
	Spec     AppUserConfigSpec `yaml:"spec" json:"spec"`
}

type RegisteredAppsManifest struct {
	Apps map[string]AppConfigSpec `yaml:"apps" json:"apps"`
}

type AppListResponse struct {
	BrokerName  string            `json:"brokerName"`
	BrokerTheme string            `json:"brokerTheme"`
	Apps        []AppDataResponse `json:"apps"`
}

type AppDataResponse struct {
	Name        string           `json:"name"`
	DisplayName string           `json:"displayName"`
	Description string           `json:"description"`
	Icon        string           `json:"icon"`
	LaunchURL   string           `json:"launchURL"`
	DefaultRepo string           `json:"defaultRepo"`
	DefaultTag  string           `json:"defaultTag"`
	NodeTiers   []string         `json:"nodeTiers"`
	DefaultTier string           `json:"defaultTier"`
	Params      []AppConfigParam `json:"params"`
}

type StatusResponse struct {
	Code      int                `json:"code"`
	Status    string             `json:"status"`
	PodIPs    []string           `json:"pod_ips,omitempty"`
	PodStatus *PodStatusResponse `json:"pod_status,omitempty"`
}

type PodStatusResponse struct {
	Ready   int64 `json:"ready"`
	Waiting int64 `json:"waiting"`
}

type ImageListManifestResponse struct {
	ImageSizeBytes  string   `json:"imageSizeBytes"`
	LayerID         string   `json:"layerId"`
	MediaType       string   `json:"mediaType"`
	Tag             []string `json:"tag"`
	TimeCreatedMs   string   `json:"timeCreatedMs"`
	TimeUploaadedMs string   `json:"timeUploadedMs"`
}

/*
Example JSON response:

{
	"child": [],
	"name": "my-gcr-project/my-gcr-image",
	"tags": [
		"v1.0.0",
		"latest"
	]
	"manifest": {
		"sha256:10a11e1a86a79a93b1605547847a69c8eda4c8a49b1ba77ec70d65a6e3819d3f": {
		"imageSizeBytes": "4818642378",
		"layerId": "",
		"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
		"tag": [
			"9fb8f45159a7785aed6776ff441dde3c5da5ceb4"
		],
		"timeCreatedMs": "1574540001823",
		"timeUploadedMs": "1574540009989"
		},
	}
}
*/
type ImageListResponse struct {
	Name     string                               `json:"name"`
	Tags     []string                             `json:"tags"`
	Manifest map[string]ImageListManifestResponse `json:"manifest"`
}
