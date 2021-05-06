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

variable release_channel {
  description = "Configuration options for the Release channels https://cloud.google.com/kubernetes-engine/docs/concepts/release-channels"
  default = "REGULAR"
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

variable network_policy {
  default = true
}

// Set this to a lower value if your pod cidr is constrained.
variable max_pods_per_node {
  default = 110
}

# Default node pool counts per zone
variable default_pool_machine_type {
  default = "e2-standard-4"
}
variable default_pool_initial_node_count {
  default = 2
}
variable default_pool_min_node_count {
  default = 2
}
variable default_pool_max_node_count {
  default = 10
}
variable default_pool_preemptive_nodes {
  default = false
}
variable default_pool_disk_size_gb {
  default = 100
}
variable default_pool_disk_type {
  default = "pd-standard"
}

# TURN node pool counts per zone
variable turn_pool_machine_type {
  default = "e2-highcpu-2"
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
variable turn_pool_preemptive_nodes {
  default = false
}
variable turn_pool_disk_size_gb {
  default = 50
}
variable turn_pool_disk_type {
  default = "pd-standard"
}

variable "ip_cidr_range" {
  description = "Custom IP CIDR ranges"
  type = map(string)
  default = {
    nodes = ""
    pods = ""
    services = ""
  }
}