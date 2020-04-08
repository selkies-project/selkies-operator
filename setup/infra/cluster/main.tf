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

data "google_container_engine_versions" "default" {
  location       = var.region
  version_prefix = var.kubernetes_version_prefix
}

module "broker" {
  source                 = "github.com/terraform-google-modules/terraform-google-kubernetes-engine//modules/beta-public-cluster?ref=v5.1.1"
  project_id             = var.project_id
  kubernetes_version     = data.google_container_engine_versions.default.latest_master_version
  name                   = "${var.name}-${var.region}"
  regional               = true
  region                 = var.region
  network                = var.network
  subnetwork             = length(var.subnetwork) == 0 ? "broker-${var.region}" : var.subnetwork
  ip_range_pods          = "broker-pods"
  ip_range_services      = "broker-services"
  node_metadata          = "GKE_METADATA_SERVER"
  identity_namespace     = "${var.project_id}.svc.id.goog"
  create_service_account = false
  service_account        = length(var.service_account) == 0 ? "broker@${var.project_id}.iam.gserviceaccount.com" : var.service_account
  monitoring_service     = "monitoring.googleapis.com/kubernetes"
  logging_service        = "logging.googleapis.com/kubernetes"
  http_load_balancing    = true
  network_policy         = true

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
      # Tier 1 node pool
      name               = "tier1"
      machine_type       = var.tier1_pool_machine_type
      initial_node_count = var.tier1_pool_initial_node_count
      min_count          = var.tier1_pool_min_node_count
      max_count          = var.tier1_pool_max_node_count
      local_ssd_count    = 0
      disk_size_gb       = var.tier1_pool_disk_size_gb
      disk_type          = "pd-ssd"
      image_type         = "COS"
      auto_repair        = true
      auto_upgrade       = true
      service_account    = length(var.service_account) == 0 ? "broker@${var.project_id}.iam.gserviceaccount.com" : var.service_account
      preemptible        = var.tier1_pool_preemptive_nodes
    },
    {
      # Tier 2 node pool
      name               = "tier2"
      machine_type       = var.tier2_pool_machine_type
      initial_node_count = var.tier2_pool_initial_node_count
      min_count          = var.tier2_pool_min_node_count
      max_count          = var.tier2_pool_max_node_count
      local_ssd_count    = 0
      disk_size_gb       = var.tier2_pool_disk_size_gb
      disk_type          = "pd-ssd"
      image_type         = "COS"
      auto_repair        = true
      auto_upgrade       = true
      service_account    = length(var.service_account) == 0 ? "broker@${var.project_id}.iam.gserviceaccount.com" : var.service_account
      preemptible        = var.tier2_pool_preemptive_nodes
    },
    {
      # GPU node pool - COS image type
      name               = "gpu-cos"
      machine_type       = var.gpu_pool_machine_type
      initial_node_count = var.gpu_pool_initial_node_count
      min_count          = var.gpu_pool_min_node_count
      max_count          = var.gpu_pool_max_node_count
      local_ssd_count    = 0
      disk_size_gb       = var.gpu_pool_disk_size_gb
      disk_type          = "pd-ssd"
      image_type         = "COS"
      auto_repair        = true
      auto_upgrade       = true
      service_account    = length(var.service_account) == 0 ? "broker@${var.project_id}.iam.gserviceaccount.com" : var.service_account
      preemptible        = var.gpu_pool_preemptive_nodes
      accelerator_count  = var.gpu_accelerator_count
      accelerator_type   = length(var.accelerator_type) == 0 ? lookup(local.accelerator_type_regions, var.region) : var.accelerator_type
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
    {
      # GPU node pool - Ubuntu image type
      name               = "gpu-ubuntu"
      machine_type       = var.gpu_ubuntu_pool_machine_type
      initial_node_count = var.gpu_ubuntu_pool_initial_node_count
      min_count          = var.gpu_ubuntu_pool_min_node_count
      max_count          = var.gpu_ubuntu_pool_max_node_count
      local_ssd_count    = 0
      disk_size_gb       = var.gpu_ubuntu_pool_disk_size_gb
      disk_type          = "pd-ssd"
      image_type         = "UBUNTU"
      auto_repair        = true
      auto_upgrade       = true
      service_account    = length(var.service_account) == 0 ? "broker@${var.project_id}.iam.gserviceaccount.com" : var.service_account
      preemptible        = var.gpu_ubuntu_pool_preemptive_nodes
      accelerator_count  = var.gpu_ubuntu_accelerator_count
      accelerator_type   = length(var.accelerator_type) == 0 ? lookup(local.accelerator_type_regions, var.region) : var.accelerator_type
    },
  ]

  node_pools_oauth_scopes = {
    all = []
    default-node-pool = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
    tier1 = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
    tier2 = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
    gpu-cos = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
    turn = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
    gpu-ubuntu = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
  }

  node_pools_labels = {
    all               = {}
    default-node-pool = {}
    tier1 = {
      # updated by node init daemonset when finished.
      "app.broker/initialized" = "false"

      # Used to set pod affinity
      "app.broker/tier" = "tier1"
    }
    tier2 = {
      # updated by node init daemonset when finished.
      "app.broker/initialized" = "false"

      # Used to set pod affinity
      "app.broker/tier" = "tier2"
    }
    gpu-cos = {
      # updated by node init daemonset when finished.
      "app.broker/initialized" = "false"

      # updated by gpu driver installer to true when finished.
      "cloud.google.com/gke-accelerator-initialized" = "false"

      # Used to set pod affinity
      "app.broker/tier" = "gpu-cos"
    }
    turn = {
      "app.broker/gke-turn" = "true"
    }
    gpu-ubuntu = {
      # updated by node init daemonset when finished.
      "app.broker/initialized" = "false"

      # updated by gpu driver installer to true when finished.
      "cloud.google.com/gke-accelerator-initialized" = "false"

      # Used to set pod affinity
      "app.broker/tier" = "gpu-ubuntu"
    }
  }

  node_pools_metadata = {
    all               = {}
    default-node-pool = {}
    tier1             = {}
    tier2             = {}
    gpu-cos           = {}
    turn              = {}
    gpu-ubuntu        = {}
  }

  node_pools_taints = {
    all               = []
    default-node-pool = []
    tier1 = [
      {
        # Taint to be removed when node init daemonset completes.
        key    = "app.broker/node-init"
        value  = true
        effect = "NO_SCHEDULE"
      },
      {
        # Repel pods without the tier toleration.
        key    = "app.broker/tier"
        value  = "tier1"
        effect = "NO_SCHEDULE"
      },
    ]
    tier2 = [
      {
        # Taint to be removed when node init daemonset completes.
        key    = "app.broker/node-init"
        value  = true
        effect = "NO_SCHEDULE"
      },
      {
        # Repel pods without the tier toleration.
        key    = "app.broker/tier"
        value  = "tier2"
        effect = "NO_SCHEDULE"
      },
    ]
    gpu-cos = [
      {
        # Taint to be removed when node init daemonset completes.
        key    = "app.broker/node-init"
        value  = true
        effect = "NO_SCHEDULE"
      },
      {
        # Repel pods without the tier toleration.
        key    = "app.broker/tier"
        value  = "gpu-cos"
        effect = "NO_SCHEDULE"
      },
      {
        # Removed when GPU driver installer daemonset completes.
        key    = "cloud.google.com/gke-accelerator-init"
        value  = "true"
        effect = "NO_SCHEDULE"
      },
    ]
    turn = [
      {
        key    = "app.broker/gke-turn"
        value  = "true"
        effect = "NO_SCHEDULE"
      },
    ]
    gpu-ubuntu = [
      {
        # Taint to be removed when node init daemonset completes.
        key    = "app.broker/node-init"
        value  = true
        effect = "NO_SCHEDULE"
      },
      {
        # Repel pods without the tier toleration.
        key    = "app.broker/tier"
        value  = "gpu-ubuntu"
        effect = "NO_SCHEDULE"
      },
      {
        # Removed when GPU driver installer daemonset completes.
        key    = "cloud.google.com/gke-accelerator-init"
        value  = "true"
        effect = "NO_SCHEDULE"
      },
    ]
  }

  node_pools_tags = {
    all               = []
    default-node-pool = []
    tier1             = []
    tier2             = []
    gpu-cos           = []
    turn              = ["gke-turn"]
    gpu-ubuntu        = []
  }
}
