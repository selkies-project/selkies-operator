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

resource "google_compute_subnetwork" "broker-west" {
  name          = "${var.name}-west"
  ip_cidr_range = "10.2.0.0/16"
  region        = "us-west1"
  network       = google_compute_network.broker.self_link

  secondary_ip_range = [
    {
      range_name    = "${var.name}-pods"
      ip_cidr_range = "172.16.0.0/16"
    },
    {
      range_name    = "${var.name}-pods-staging"
      ip_cidr_range = "172.17.0.0/16"
    },
    {
      range_name    = "${var.name}-pods-dev"
      ip_cidr_range = "172.18.0.0/16"
    },
    {
      range_name    = "${var.name}-services"
      ip_cidr_range = "192.168.0.0/24"
    },
    {
      range_name    = "${var.name}-services-staging"
      ip_cidr_range = "192.168.1.0/24"
    },
    {
      range_name    = "${var.name}-services-dev"
      ip_cidr_range = "192.168.2.0/24"
    }
  ]
}

data "google_container_engine_versions" "us-west1" {
  location       = "us-west1"
  version_prefix = var.kubernetes_version_prefix
}

module "broker-west" {
  source                 = "github.com/terraform-google-modules/terraform-google-kubernetes-engine//modules/beta-public-cluster?ref=v5.1.1"
  project_id             = var.project_id
  kubernetes_version     = data.google_container_engine_versions.us-west1.latest_master_version
  name                   = "${var.name}-us-west1"
  regional               = true
  region                 = "us-west1"
  network                = google_compute_network.broker.name
  subnetwork             = google_compute_subnetwork.broker-west.name
  ip_range_pods          = "broker-pods"
  ip_range_services      = "broker-services"
  node_metadata          = "GKE_METADATA_SERVER"
  identity_namespace     = "${var.project_id}.svc.id.goog"
  create_service_account = false
  service_account        = google_service_account.cluster_service_account.email
  monitoring_service     = "monitoring.googleapis.com/kubernetes"
  logging_service        = "logging.googleapis.com/kubernetes"
  http_load_balancing    = true
  network_policy         = true

  # Zones must be compatible with the chosen accelerator_type in the gpu-* node pools. 
  zones = ["us-west1-a", "us-west1-b"]

  node_pools = [
    {
      # Default node pool
      name            = "default-node-pool"
      machine_type    = "n1-standard-4"
      min_count       = 1
      max_count       = 100
      local_ssd_count = 0
      disk_size_gb    = 100
      disk_type       = "pd-standard"
      image_type      = "COS"
      auto_repair     = true
      auto_upgrade    = true
      service_account = google_service_account.cluster_service_account.email
      preemptible     = false
    },
    {
      # Tier 1 node pool
      name            = "tier1"
      machine_type    = "n1-standard-8"
      min_count       = 1
      max_count       = 100
      local_ssd_count = 0
      disk_size_gb    = 512
      disk_type       = "pd-ssd"
      image_type      = "COS"
      auto_repair     = true
      auto_upgrade    = true
      service_account = google_service_account.cluster_service_account.email
      preemptible     = false
    },
    {
      # Tier 2 node pool
      name            = "tier2"
      machine_type    = "n1-highcpu-32"
      min_count       = 0
      max_count       = 25
      local_ssd_count = 0
      disk_size_gb    = 512
      disk_type       = "pd-ssd"
      image_type      = "COS"
      auto_repair     = true
      auto_upgrade    = true
      service_account = google_service_account.cluster_service_account.email
      preemptible     = false
    },
    {
      # GPU node pool - COS image type
      name               = "gpu-cos"
      machine_type       = "n1-standard-16"
      initial_node_count = 1
      min_count          = 1
      max_count          = 10
      local_ssd_count    = 0
      disk_size_gb       = 512
      disk_type          = "pd-ssd"
      image_type         = "COS"
      auto_repair        = true
      auto_upgrade       = true
      service_account    = google_service_account.cluster_service_account.email
      preemptible        = false
      accelerator_count  = 1
      accelerator_type   = "nvidia-tesla-t4"
    },
    {
      # TURN node pool for apps with NAT traversal
      name               = "turn"
      machine_type       = "n1-standard-2"
      initial_node_count = 1
      min_count          = 1
      max_count          = 10
      local_ssd_count    = 0
      disk_size_gb       = 512
      disk_type          = "pd-standard"
      image_type         = "COS"
      auto_repair        = true
      auto_upgrade       = true
      service_account    = google_service_account.cluster_service_account.email
      preemptible        = false
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
  }

  node_pools_metadata = {
    all               = {}
    default-node-pool = {}
    tier1             = {}
    tier2             = {}
    gpu-cos           = {}
    turn              = {}
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
  }

  node_pools_tags = {
    all               = []
    default-node-pool = []
    tier1             = []
    tier2             = []
    gpu-cos           = []
    turn              = ["gke-turn"]
  }
}
