# Copyright 2022 The Selkies Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
variable "gcp_service_list" {
  description ="The list of apis necessary for the project"
  type = list(string)
  default = [
    "containerfilesystem.googleapis.com",
    "artifactregistry.googleapis.com"
  ]
}


variable project_id {}
variable region {}

variable name {
  default = "broker"
}

# Tier 1 COS node pool parameters
variable tier3_pool_enabled {
  default = true
}
variable tier3_pool_machine_type {
  default = "e2-standard-8"
}
variable tier3_pool_initial_node_count {
  default = 1
}
variable tier3_pool_min_node_count {
  default = 0
}
variable tier3_pool_max_node_count {
  default = 10
}
variable tier3_pool_preemptive_nodes {
  default = false
}
variable tier3_pool_disk_size_gb {
  default = 100
}
variable tier3_pool_disk_type {
  default = "pd-balanced"
}
variable tier3_pool_ephemeral_storage_ssd_count {
  description = "use local-ssd for ephemeral container storage. NOTE: requires either n1, n2 or n2d instance types."
  default = 0
}

# Tier 1 Ubuntu node pool parameters
variable tier3_ubuntu_pool_enabled {
  default = false
}
variable tier3_ubuntu_pool_machine_type {
  default = "e2-standard-8"
}
variable tier3_ubuntu_pool_initial_node_count {
  default = 1
}
variable tier3_ubuntu_pool_min_node_count {
  default = 0
}
variable tier3_ubuntu_pool_max_node_count {
  default = 10
}
variable tier3_ubuntu_pool_preemptive_nodes {
  default = false
}
variable tier3_ubuntu_pool_disk_size_gb {
  default = 100
}
variable tier3_ubuntu_pool_disk_type {
  default = "pd-balanced"
}
variable tier3_ubuntu_pool_ephemeral_storage_ssd_count {
  description = "use local-ssd for ephemeral container storage. NOTE: requires either n1, n2 or n2d instance types."
  default = 0
}


