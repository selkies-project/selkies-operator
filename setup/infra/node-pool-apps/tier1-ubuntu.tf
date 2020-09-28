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

resource "google_container_node_pool" "tier1-ubuntu" {
  count              = var.tier1_ubuntu_pool_enabled ? 1 : 0
  name               = "tier1-ubuntu"
  location           = var.region
  cluster            = data.google_container_cluster.broker.name
  initial_node_count = var.tier1_ubuntu_pool_initial_node_count

  node_config {
    preemptible  = var.tier1_ubuntu_pool_preemptive_nodes
    machine_type = var.tier1_ubuntu_pool_machine_type

    service_account = data.google_service_account.broker_cluster.email

    disk_size_gb = var.tier1_ubuntu_pool_disk_size_gb
    disk_type    = "pd-ssd"

    image_type = "UBUNTU"

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]

    metadata = {
      cluster_name             = data.google_container_cluster.broker.name
      node_pool                = "tier1-ubuntu"
      disable-legacy-endpoints = "true"
    }

    labels = {
      cluster_name = data.google_container_cluster.broker.name
      node_pool    = "tier1-ubuntu"

      # updated by node init daemonset when finished.
      "app.broker/initialized" = "false"

      # Used to set pod affinity
      "app.broker/tier" = "tier1-ubuntu"
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
        value  = "tier1-ubuntu"
        effect = "NO_SCHEDULE"
      },
    ]
  }

  management {
    auto_repair  = true
    auto_upgrade = true
  }

  autoscaling {
    min_node_count = var.tier1_ubuntu_pool_min_node_count
    max_node_count = var.tier1_ubuntu_pool_max_node_count
  }

  // node labels and taints are modified dynamically by the node init containers
  // ignore changes so that Terraform doesn't try to undo their modifications.
  lifecycle {
    ignore_changes = [
      node_config[0].labels,
      node_config[0].taint,
      autoscaling[0].min_node_count,
      autoscaling[0].max_node_count
    ]
  }
}