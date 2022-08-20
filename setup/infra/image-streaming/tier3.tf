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

resource "google_container_node_pool" "tier3" {
  provider           = google-beta
  count              = var.tier3_pool_enabled ? 1 : 0
  name               = "tier3"
  location           = var.region
  cluster            = data.google_container_cluster.broker.name
  initial_node_count = var.tier3_pool_initial_node_count

  node_config {
    preemptible  = var.tier3_pool_preemptive_nodes
    machine_type = var.tier3_pool_machine_type

    service_account = data.google_service_account.broker_cluster.email

    disk_size_gb = var.tier3_pool_disk_size_gb
    disk_type    = var.tier3_pool_disk_type

    ephemeral_storage_config {
      local_ssd_count = var.tier3_pool_ephemeral_storage_ssd_count
    }

    image_type = "COS_CONTAINERD"
    # gcfs_config - (Optional) Parameters for the Google Container Filesystem (GCFS). If unspecified,
    #  GCFS will not be enabled on the node pool.
    #  When enabling this feature you must specify 
    # image_type = "COS_CONTAINERD" and node_version from 
    # GKE versions 1.19 or later to use it. 
    # For GKE versions 1.19, 1.20, and 1.21, 
    # the recommended minimum node_version would be 
    # 1.19.15-gke.1300, 1.20.11-gke.1300, and 1.21.5-gke.1300 respectively.
    #  A machine_type that has more than 16 GiB of memory is also recommended. 
    # GCFS must be enabled in order to use image streaming. 
    # Open ISSUE
    # https://github.com/hashicorp/terraform-provider-google/issues/10509
    gcfs_config {
      enabled = true
    }
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]

    metadata = {
      cluster_name             = data.google_container_cluster.broker.name
      node_pool                = "tier3"
      disable-legacy-endpoints = "true"
    }

    labels = {
      cluster_name = data.google_container_cluster.broker.name
      node_pool    = "tier3"

      # updated by node init daemonset when finished.
      "app.broker/initialized" = "false"

      # Used to set pod affinity
      "app.broker/tier" = "tier3"
    }

    taint = [
      {
        # Taint to be removed when node init daemonset completes.
        key    = "app.broker/node-init"
        value  = true
        effect = "NO_SCHEDULE"
      },
      {
        # Repel pods without the tier toleration.
        key    = "app.broker/tier"
        value  = "tier3"
        effect = "NO_SCHEDULE"
      },
    ]
  }

  management {
    auto_repair  = true
    auto_upgrade = true
  }

  autoscaling {
    min_node_count = var.tier3_pool_min_node_count
    max_node_count = var.tier3_pool_max_node_count
  }

  // node labels and taints are modified dynamically by the node init containers
  // ignore changes so that Terraform doesn't try to undo their modifications.
  lifecycle {
    ignore_changes = [
      node_config[0].labels,
      node_config[0].taint
    ]
  }
}
