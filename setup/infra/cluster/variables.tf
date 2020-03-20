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
variable kubernetes_version_prefix {}
variable region {}

variable network {
  default = "broker"
}

variable subnetwork {
  # If not specified, will use: "broker-${var.region}"
  default = ""
}

variable service_account {
  # If not specified, will use: broker@${var.project_id}.iam.gserviceaccount.com
  default = ""
}

variable zones {
  type    = list
  default = []
}

variable name {
  default = "broker"
}

# Default node pool counts per zone
variable default_pool_machine_type {
  default = "n1-standard-4"
}
variable default_pool_initial_node_count {
  default = 1
}
variable default_pool_min_node_count {
  default = 1
}
variable default_pool_max_node_count {
  default = 10
}

# Tier 1 node pool counts per zone
variable tier1_pool_machine_type {
  default = "n1-standard-8"
}
variable tier1_pool_initial_node_count {
  default = 1
}
variable tier1_pool_min_node_count {
  default = 1
}
variable tier1_pool_max_node_count {
  default = 10
}

# Tier 2 node pool counts per zone
variable tier2_pool_machine_type {
  default = "n1-highcpu-32"
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

# GPU node pool counts per zone
variable accelerator_type {
  # will use value from accelerator_type_regions if not given.
  default = ""
}
variable gpu_pool_machine_type {
  default = "n1-standard-16"
}
variable gpu_pool_initial_node_count {
  default = 1
}
variable gpu_pool_min_node_count {
  default = 1
}
variable gpu_pool_max_node_count {
  default = 10
}

variable gpu_ubuntu_pool_machine_type {
  default = "n1-standard-16"
}
variable gpu_ubuntu_pool_initial_node_count {
  default = 0
}
variable gpu_ubuntu_pool_min_node_count {
  default = 0
}
variable gpu_ubuntu_pool_max_node_count {
  default = 10
}

# TURN node pool counts per zone
variable turn_pool_machine_type {
  default = "n1-standard-2"
}
variable turn_pool_initial_node_count {
  default = 1
}
variable turn_pool_min_node_count {
  default = 1
}
variable turn_pool_max_node_count {
  default = 10
}