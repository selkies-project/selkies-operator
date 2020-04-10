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

module "broker" {
  source                 = "github.com/terraform-google-modules/terraform-google-kubernetes-engine//modules/beta-public-cluster?ref=v8.1.0"
  project_id             = var.project_id
  release_channel        = "REGULAR"
  name                   = "${var.name}-${var.region}"
  regional               = true
  region                 = var.region
  network                = var.network
  subnetwork             = length(var.subnetwork) == 0 ? "broker-${var.region}" : var.subnetwork
  ip_range_pods          = "broker-pods"
  ip_range_services      = "broker-services"
  node_metadata          = "GKE_METADATA_SERVER"
  create_service_account = false
  service_account        = length(var.service_account) == 0 ? "broker@${var.project_id}.iam.gserviceaccount.com" : var.service_account

  # Zones must be compatible with the chosen accelerator_type in the gpu-* node pools.
  zones = length(var.zones) == 0 ? lookup(local.cluster_node_zones, var.region) : var.zones

  node_pools = [
    {
      # Default node pool
      name               = "default-node-pool"
      machine_type       = var.default_pool_machine_type
      initial_node_count = var.default_pool_initial_node_count
      min_count          = var.default_pool_min_node_count
      max_count          = var.default_pool_max_node_count
      local_ssd_count    = 0
      disk_size_gb       = var.default_pool_disk_size_gb
      disk_type          = "pd-standard"
      image_type         = "COS"
      auto_repair        = true
      auto_upgrade       = true
      service_account    = length(var.service_account) == 0 ? "broker@${var.project_id}.iam.gserviceaccount.com" : var.service_account
      preemptible        = var.default_pool_preemptive_nodes
    },
    {
      # TURN node pool for apps with NAT traversal
      name               = "turn"
      machine_type       = var.turn_pool_machine_type
      initial_node_count = var.turn_pool_initial_node_count
      min_count          = var.turn_pool_min_node_count
      max_count          = var.turn_pool_max_node_count
      local_ssd_count    = 0
      disk_size_gb       = var.turn_pool_disk_size_gb
      disk_type          = "pd-standard"
      image_type         = "COS"
      auto_repair        = true
      auto_upgrade       = true
      service_account    = length(var.service_account) == 0 ? "broker@${var.project_id}.iam.gserviceaccount.com" : var.service_account
      preemptible        = var.turn_pool_preemptive_nodes
    },
  ]

  node_pools_oauth_scopes = {
    all = []
    default-node-pool = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
    turn = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
  }

  node_pools_labels = {
    all               = {}
    default-node-pool = {}
    turn = {
      "app.broker/gke-turn" = "true"
    }
  }

  node_pools_metadata = {
    all               = {}
    default-node-pool = {}
    turn              = {}
  }

  node_pools_taints = {
    all               = []
    default-node-pool = []
    turn = [
      {
        key    = "app.broker/gke-turn"
        value  = "true"
        effect = "NO_SCHEDULE"
      },
    ]
  }

  node_pools_tags = {
    all               = []
    default-node-pool = []
    turn              = ["gke-turn"]
  }
}
