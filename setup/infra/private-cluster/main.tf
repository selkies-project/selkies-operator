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
  source                    = "terraform-google-modules/kubernetes-engine/google//modules/beta-private-cluster-update-variant"
  version                   = "21.2.0"
  project_id                = var.project_id
  release_channel           = var.release_channel
  name                      = "${var.name}-${var.region}"
  regional                  = true
  region                    = var.region
  network                   = data.google_compute_network.broker.name
  subnetwork                = google_compute_subnetwork.broker.name
  ip_range_pods             = google_compute_subnetwork.broker.secondary_ip_range[0].range_name
  ip_range_services         = google_compute_subnetwork.broker.secondary_ip_range[1].range_name
  node_metadata             = "GKE_METADATA_SERVER"
  create_service_account    = false
  service_account           = length(var.service_account) == 0 ? "broker@${var.project_id}.iam.gserviceaccount.com" : var.service_account
  remove_default_node_pool  = true
  network_policy            = var.network_policy
  master_ipv4_cidr_block    = var.ip_cidr_range.master != "" ? var.ip_cidr_range.master : local.default_ip_cidr_range.master
  enable_private_endpoint   = false
  enable_private_nodes      = true
  default_max_pods_per_node = var.max_pods_per_node
  gce_pd_csi_driver         = true
  config_connector          = true

  depends_on = [
    google_compute_subnetwork.broker
  ]
  // Enable network metering to track per-user traffic.
  // https://cloud.google.com/kubernetes-engine/docs/how-to/cluster-usage-metering#enable-network-egress-metering
  enable_network_egress_export = true

  // Save consumption data to BigQuery
  enable_resource_consumption_export = true

  # Zones must be compatible with the chosen accelerator_type in the gpu-* node pools.
  zones = length(var.zones) == 0 ? lookup(local.cluster_node_zones, var.region) : var.zones

  // Add firewall rules for admission controllers with private clusters
  // https://istio.io/latest/docs/setup/platform-setup/gke/
  // https://istio.io/latest/docs/ops/deployment/requirements/#ports-used-by-istio
  add_cluster_firewall_rules = true
  firewall_inbound_ports = [
    "443", // cnrm webhook
    "8443",
    "9443",
    "15010", // istiod
    "15012", // istiod
    "15014", // istiod
    "15017", // istiod
    "10250"
  ]

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
      disk_type          = var.default_pool_disk_type
      image_type         = "COS"
      auto_repair        = true
      auto_upgrade       = true
      service_account    = length(var.service_account) == 0 ? "broker@${var.project_id}.iam.gserviceaccount.com" : var.service_account
      preemptible        = var.default_pool_preemptive_nodes
    },
  ]

  node_pools_oauth_scopes = {
    all = []
    default-node-pool = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
  }

  node_pools_labels = {
    all = {}
    default-node-pool = {
      "cloud.google.com/gke-private-cluster" = "true"
    }
  }

  node_pools_metadata = {
    all               = {}
    default-node-pool = {}
  }

  node_pools_taints = {
    all               = []
    default-node-pool = []
  }

  node_pools_tags = {
    all               = []
    default-node-pool = []
  }
}