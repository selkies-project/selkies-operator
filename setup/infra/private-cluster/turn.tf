/**
 * Copyright 2020 Google LLC
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

// Normally the TURN nodes are part of the cluster, run as a node pool.
// For Private CLusters, the nodes do not have external IPs, so the TURN nodes are run in a managed instance group instead.

// Lookup cookie secret used for TURN shared secret.
data "google_secret_manager_secret_version" "broker-cookie-secret" {
  secret = "broker-cookie-secret"
}

locals {
  coturn_image       = "gcr.io/${var.project_id}/kube-pod-broker-coturn:latest"
  coturn_web_image   = "gcr.io/${var.project_id}/kube-pod-broker-coturn-web:latest"
  turn_shared_secret = data.google_secret_manager_secret_version.broker-cookie-secret.secret_data

  // Update this based on auth mechanism
  auth_header_name = "x-goog-authenticated-user-email"

  // Default the REALM to the Cloud Endpoints DNS name
  turn_realm = "broker.endpoints.${var.project_id}.cloud.goog"
}

module "turn-cos-nodes" {
  source                = "./turn-mig"
  instance_count        = var.turn_pool_instance_count
  project_id            = var.project_id
  subnetwork            = google_compute_subnetwork.broker.self_link
  machine_type          = var.turn_pool_machine_type
  preemptible           = var.turn_pool_preemptive_nodes
  region                = var.region
  zones                 = local.cluster_node_zones[var.region]
  name                  = "selkies-turn-${var.region}"
  disk_size_gb          = var.turn_pool_disk_size_gb
  disk_type             = var.turn_pool_disk_type
  scopes                = ["https://www.googleapis.com/auth/cloud-platform"]
  service_account       = length(var.service_account) == 0 ? "broker@${var.project_id}.iam.gserviceaccount.com" : var.service_account
  cloud_init_custom_var = "${local.turn_shared_secret},${local.turn_realm},${local.auth_header_name},${local.coturn_image},${local.coturn_web_image}"
}