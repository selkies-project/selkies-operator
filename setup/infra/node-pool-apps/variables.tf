/**
 * Copyright 2019 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

variable project_id {}
variable region {}

variable name {
  default = "broker"
}

# Tier 1 COS node pool parameters
variable tier1_pool_enabled {
  default = true
}
variable tier1_pool_machine_type {
  default = "e2-standard-4"
}
variable tier1_pool_initial_node_count {
  default = 1
}
variable tier1_pool_min_node_count {
  default = 0
}
variable tier1_pool_max_node_count {
  default = 10
}
variable tier1_pool_preemptive_nodes {
  default = false
}
variable tier1_pool_disk_size_gb {
  default = 100
}
variable tier1_pool_disk_type {
  default = "pd-balanced"
}

# Tier 1 Ubuntu node pool parameters
variable tier1_ubuntu_pool_enabled {
  default = false
}
variable tier1_ubuntu_pool_machine_type {
  default = "e2-standard-4"
}
variable tier1_ubuntu_pool_initial_node_count {
  default = 1
}
variable tier1_ubuntu_pool_min_node_count {
  default = 0
}
variable tier1_ubuntu_pool_max_node_count {
  default = 10
}
variable tier1_ubuntu_pool_preemptive_nodes {
  default = false
}
variable tier1_ubuntu_pool_disk_size_gb {
  default = 100
}
variable tier1_ubuntu_pool_disk_type {
  default = "pd-balanced"
}

# Tier 2 node pool parameters
variable tier2_pool_enabled {
  default = false
}
variable tier2_pool_machine_type {
  default = "e2-highcpu-32"
}
variable tier2_pool_initial_node_count {
  default = 0
}
variable tier2_pool_min_node_count {
  default = 0
}
variable tier2_pool_max_node_count {
  default = 10
}
variable tier2_pool_preemptive_nodes {
  default = false
}
variable tier2_pool_disk_size_gb {
  default = 100
}
variable tier2_pool_disk_type {
  default = "pd-balanced"
}