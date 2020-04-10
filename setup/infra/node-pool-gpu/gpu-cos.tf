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

resource "google_container_node_pool" "gpu-cos" {
  count              = var.gpu_cos_pool_enabled ? 1 : 0
  name               = "gpu-cos"
  location           = var.region
  cluster            = data.google_container_cluster.broker.name
  initial_node_count = var.gpu_cos_pool_initial_node_count

  node_config {
    preemptible  = var.gpu_cos_pool_preemptive_nodes
    machine_type = var.gpu_cos_pool_machine_type

    service_account = data.google_service_account.broker_cluster.email

    disk_size_gb = var.gpu_cos_pool_disk_size_gb
    disk_type    = "pd-ssd"

    image_type = "COS"

    guest_accelerator {
      count = 1
      type  = length(var.gpu_cos_accelerator_type) == 0 ? lookup(local.accelerator_type_regions, var.region) : var.gpu_cos_accelerator_type
    }

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]

    metadata = {
      cluster_name             = data.google_container_cluster.broker.name
      node_pool                = "gpu-cos"
      disable-legacy-endpoints = "true"
    }

    labels = {
      cluster_name = data.google_container_cluster.broker.name
      node_pool    = "gpu-cos"

      # updated by node init daemonset when finished.
      "app.broker/initialized" = "false"

      # Used to set pod affinity
      "app.broker/tier" = "gpu-cos"

      # updated by gpu driver installer to true when finished.
      "cloud.google.com/gke-accelerator-initialized" = "false"
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
  }

  management {
    auto_repair  = true
    auto_upgrade = true
  }

  autoscaling {
    min_node_count = var.gpu_cos_pool_min_node_count
    max_node_count = var.gpu_cos_pool_max_node_count
  }
}