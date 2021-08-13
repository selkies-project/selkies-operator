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

# GPU COS node pool parameters
variable gpu_cos_pool_enabled {
  default = true
}
variable gpu_cos_accelerator_type {
  # will use value from accelerator_type_regions if not given.
  default = ""
}
variable gpu_cos_pool_machine_type {
  default = "n1-standard-8"
}
variable gpu_cos_pool_initial_node_count {
  default = 1
}
variable gpu_cos_pool_min_node_count {
  default = 0
}
variable gpu_cos_pool_max_node_count {
  default = 10
}
variable gpu_cos_pool_preemptive_nodes {
  default = false
}
variable gpu_cos_pool_disk_size_gb {
  default = 100
}
variable gpu_cos_pool_disk_type {
  default = "pd-balanced"
}
variable gpu_cos_pool_ephemeral_storage_ssd_count {
  description = "use local-ssd for ephemeral container storage. NOTE: requires either n1, n2 or n2d instance types."
  default = 0
}

# GPU Ubuntu node pool parameters
variable gpu_ubuntu_pool_enabled {
  default = true
}
variable gpu_ubuntu_pool_machine_type {
  default = "n1-standard-8"
}
variable gpu_ubuntu_accelerator_type {
  # will use value from accelerator_type_regions if not given.
  default = ""
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
variable gpu_ubuntu_pool_preemptive_nodes {
  default = false
}
variable gpu_ubuntu_pool_disk_size_gb {
  default = 100
}
variable gpu_ubuntu_pool_disk_type {
  default = "pd-balanced"
}
variable gpu_ubuntu_pool_ephemeral_storage_ssd_count {
  description = "use local-ssd for ephemeral container storage. NOTE: requires either n1, n2 or n2d instance types."
  default = 0
}