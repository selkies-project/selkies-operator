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

const DefaultBrokerNamespace = "pod-broker-system"

const ApiVersion = "gcp.solutions/v1"
const BrokerAppConfigKind = "BrokerAppConfig"
const BrokerAppConfigApiGroup = "brokerappconfigs.v1.gcp.solutions"
const BrokerAppUserConfigKind = "BrokerAppUserConfig"
const BrokerAppUserConfigApiGroup = "brokerappuserconfigs.v1.gcp.solutions"

type AppType string

const AppTypeStatefulSet AppType = "statefulset"
const AppTypeDeployment AppType = "deployment"
const AppTypeAll AppType = "all"

const DefaultDeploymentReplicas = 2

const BrokerCommonBuildSourceBaseDirStatefulSetUser = "/opt/broker/buildsrc/statefulset/user"
const BrokerCommonBuildSourceBaseDirStatefulSetApp = "/opt/broker/buildsrc/statefulset/app"
const BrokerCommonBuildSourceBaseDirDeploymentUser = "/opt/broker/buildsrc/deployment/user"
const BrokerCommonBuildSourceBaseDirDeploymentApp = "/opt/broker/buildsrc/deployment/app"
const BrokerCommonBuildSourceBaseDirApp = "/opt/broker/buildsrc/deployment/app"
const BundleSourceBaseDir = "/var/run/buildsrc/apps"
const UserBundleSourceBaseDir = "/var/run/buildsrc/user"
const BuildSourceBaseDir = "/var/run/build"
const BuildSourceBaseDirNS = "/var/run/buildns"
const BuildSourceBaseDirUser = "/var/run/buildusr"
const RegisteredAppsManifestJSONFile = "/var/run/buildsrc/apps.json"
const AppUserConfigBaseDir = "/var/run/userconfig"
const AppUserConfigJSONFile = "app-user-config.json"
const JSONPatchObjectTypes = "jsonpatch-deploy-last-applied-resources.yaml"

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
	ProjectID                 string
	ClientID                  string
	AppSpec                   AppConfigSpec
	App                       string
	AppMetadata               map[string]string
	AppUserConfig             AppUserConfigSpec
	ImageRepo                 string
	ImageTag                  string
	NodeTier                  NodeTierSpec
	Domain                    string
	User                      string
	Username                  string
	CookieValue               string
	ID                        string
	FullName                  string
	ServiceName               string
	Resources                 []string
	Patches                   []string
	JSONPatchesService        []string
	JSONPatchesVirtualService []string
	JSONPatchesDeploy         []string
	JSONPatchesNamespace      []string
	JSONPatchesServiceAccount []string
	JSONPatchesNetworkPolicy  []string
	UserParams                map[string]string
	AppParams                 map[string]string
	SysParams                 map[string]string
	NetworkPolicyData         NetworkPolicyTemplateData
	Timestamp                 string
	Region                    string
	Editable                  bool
	PullSecrets               []string
}

type NodeResource struct {
	CPU              interface{} `yaml:"cpu,omitempty" json:"cpu,omitempty,string"`
	Memory           interface{} `yaml:"memory,omitempty" json:"memory,omitempty,string"`
	EphemeralStorage interface{} `yaml:"ephemeral-storage,omitempty" json:"ephemeral-storage,omitempty,string"`
}

type NodeResourceRequestSpec struct {
	Requests *NodeResource `yaml:"requests,omitempty" json:"requests,omitempty"`
	Limits   *NodeResource `yaml:"limits,omitempty" json:"limits,omitempty"`
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

type AuthZUsersSpec struct {
	BundleSpec
}

type UserBundleSpec struct {
	BundleSpec
}

type AppConfigParam struct {
	Name        string `yaml:"name" json:"name"`
	DisplayName string `yaml:"displayName" json:"displayName"`
	Type        string `yaml:"type" json:"type"`
	Default     string `yaml:"default" json:"default"`
	Regexp      string `yaml:"regexp" json:"regexp"`
}

type AppImageSpec struct {
	Name    string `yaml:"name" json:"name"`
	OldRepo string `yaml:"oldRepo" json:"oldRepo"`
	NewRepo string `yaml:"newRepo" json:"newRepo"`
	NewTag  string `yaml:"newTag" json:"newTag"`
	Digest  string `yaml:"digest,omitempty" json:"digest,omitempty"`
}

type AppEnvSpec struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

type ShutdownHookSpec struct {
	Selector  string `yaml:"selector" json:"selector"`
	Container string `yaml:"container" json:"container"`
	Command   string `yaml:"command" json:"command"`
}

type DeploymentTypeSpec struct {
	Replicas *int   `yaml:"replicas" json:"replicas"`
	Selector string `yaml:"selector" json:"selector"`
}

type AppConfigSpec struct {
	Type                 AppType                 `yaml:"type" json:"type"`
	Name                 string                  `yaml:"name" json:"name"`
	Metadata             map[string]string       `yaml:"metadata" json:"metadata"`
	DisplayName          string                  `yaml:"displayName" json:"displayName"`
	Description          string                  `yaml:"description" json:"description"`
	Icon                 string                  `yaml:"icon,omitempty" json:"icon,omitempty"`
	LaunchURL            string                  `yaml:"launchURL,omitempty" json:"launchURL,omitempty"`
	Disabled             bool                    `yaml:"disabled" json:"disabled"`
	Version              string                  `yaml:"version" json:"version"`
	Deployment           DeploymentTypeSpec      `yaml:"deployment" json:"deployment"`
	Bundle               BundleSpec              `yaml:"bundle" json:"bundle"`
	DefaultRepo          string                  `yaml:"defaultRepo" json:"defaultRepo"`
	DefaultTag           string                  `yaml:"defaultTag" json:"defaultTag"`
	Images               map[string]AppImageSpec `yaml:"images,omitempty" json:"images,omitempty"`
	NodeTiers            []NodeTierSpec          `yaml:"nodeTiers,omitempty" json:"nodeTiers,omitempty"`
	DefaultTier          string                  `yaml:"defaultTier,omitempty" json:"defaultTier,omitempty"`
	ServiceName          string                  `yaml:"serviceName" json:"serviceName"`
	UserParams           []AppConfigParam        `yaml:"userParams" json:"userParams"`
	EnableUserConfigAuth bool                    `yaml:"enableUserConfigAuth" json:"enableUserConfigAuth"`
	UserWritableFields   []string                `yaml:"userWritableFields" json:"userWritableFields"`
	UserWritableParams   []string                `yaml:"userWritableParams" json:"userWritableParams"`
	AppParams            []AppConfigParam        `yaml:"appParams" json:"appParams"`
	AppEnv               []AppEnvSpec            `yaml:"appEnv" json:"appEnv"`
	ShutdownHooks        []ShutdownHookSpec      `yaml:"shutdownHooks" json:"shutdownHooks"`
	Editors              []string                `yaml:"editors" json:"editors"`
	AuthorizedUsers      []string                `yaml:"authorizedUsers" json:"authorizedUsers"`
	Authorization        AuthZUsersSpec          `yaml:"authorization" json:"authorization"`
	DisableOptions       bool                    `yaml:"disableOptions" json:"disableOptions"`
	UserBundles          []UserBundleSpec        `yaml:"userBundles" json:"userBundles"`
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
	Apps              map[string]AppConfigSpec  `yaml:"apps" json:"apps"`
	NetworkPolicyData NetworkPolicyTemplateData `yaml:"networkPolicyData" json:"networkPolicyData"`
}

type AppListResponse struct {
	BrokerName   string            `json:"brokerName"`
	BrokerTheme  string            `json:"brokerTheme"`
	BrokerRegion string            `json:"brokerRegion"`
	Apps         []AppDataResponse `json:"apps"`
	User         string            `json:"user"`
	LogoutURL    string            `json:"logoutURL"`
}

type AppDataResponse struct {
	Name           string            `json:"name"`
	Type           AppType           `json:"type"`
	DisplayName    string            `json:"displayName"`
	Description    string            `json:"description"`
	Icon           string            `json:"icon"`
	LaunchURL      string            `json:"launchURL"`
	DefaultRepo    string            `json:"defaultRepo"`
	DefaultTag     string            `json:"defaultTag"`
	NodeTiers      []string          `json:"nodeTiers"`
	DefaultTier    string            `json:"defaultTier"`
	Params         []AppConfigParam  `json:"params"`
	Editable       bool              `json:"editable"`
	DisableOptions bool              `json:"disableOptions"`
	Metadata       map[string]string `json:"metadata"`
}

type StatusResponse struct {
	Code              int                `json:"code"`
	Status            string             `json:"status"`
	Nodes             []string           `json:"nodes,omitempty"`
	Containers        map[string]string  `json:"containers,omitempty"`
	Images            map[string]string  `json:"images,omitempty"`
	PodIPs            []string           `json:"pod_ips,omitempty"`
	PodStatus         *PodStatusResponse `json:"pod_status,omitempty"`
	SessionKeys       []string           `json:"session_keys"`
	BrokerObjects     []string           `json:"broker_objects"`
	CreationTimestamp string             `json:"creation_timestamp"`
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

type ReservationMetadataSpec struct {
	IP           string `json:"ip"`
	SessionKey   string `json:"session_key"`
	User         string `json:"user"`
	SessionStart string `json:"session_start"`
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

type BrokerImage struct {
	Name   string `json:"name"`
	Tag    string `json:"tag"`
	Digest string `json:"digest"`
}

type NodeAddress struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}

type DockerImage struct {
	Repository string `json:"Repository"`
	Digest     string `json:"Digest"`
}

// Map of endpoint name to nodes that endpoint is deployed to.
type EndpointNodeMap map[string][]string

type EndpointNodeIPMap struct {
	EndpointName string    `json:"endpointName"`
	Nodes        []NodeIPs `json:"nodes"`
}

type NodeIPs struct {
	NodeName   string `json:"nodeName"`
	InternalIP string `json:"internalIP"`
	ExternalIP string `json:"externalIP"`
}

type EndpointNodeIPMapList struct {
	Endpoints []EndpointNodeIPMap `json:"endpoints"`
}

type ServiceClusterIP struct {
	ServiceName string `json:"serviceName"`
	ClusterIP   string `json:"clusterIP"`
}

type ServiceClusterIPList struct {
	Services []ServiceClusterIP `json:"services"`
}

type NetworkPolicyTemplateData struct {
	KubeDNSClusterIP string   `json:"kubeDNSClusterIP"`
}

type JobStatusSpec struct {
	CompletionTime string              `json:"completionTime"`
	Conditions     []map[string]string `json:"conditions"`
	StartTime      string              `json:"startTime"`
	Succeeded      int                 `json:"succeeded"`
}

type GetJobSpec struct {
	Metadata map[string]interface{} `json:"metadata"`
	Status   JobStatusSpec          `json:"status"`
}

type GCRPubSubMessage struct {
	Action string `json:"action"`
	Digest string `json:"digest"`
	Tag    string `json:"tag"`
}
