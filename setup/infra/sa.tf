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

# Service account used by the nodes.
resource "google_service_account" "cluster_service_account" {
  project      = var.project_id
  account_id   = var.name
  display_name = "${var.name} GKE cluster"
  depends_on   = [google_project_service.iam]
}

resource "google_project_iam_member" "cluster_service_account-log_writer" {
  project = google_service_account.cluster_service_account.project
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-metric_writer" {
  project = google_project_iam_member.cluster_service_account-log_writer.project
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-monitoring_viewer" {
  project = google_project_iam_member.cluster_service_account-metric_writer.project
  role    = "roles/monitoring.viewer"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-stackdriver-metadata" {
  project = google_project_iam_member.cluster_service_account-metric_writer.project
  role    = "roles/stackdriver.resourceMetadata.writer"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-iap-user" {
  project = google_project_iam_member.cluster_service_account-metric_writer.project
  role    = "roles/iap.httpsResourceAccessor"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-gke-admin" {
  project = google_project_iam_member.cluster_service_account-metric_writer.project
  role    = "roles/container.clusterAdmin"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-storage-admin" {
  project = google_project_iam_member.cluster_service_account-metric_writer.project
  role    = "roles/storage.admin"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

resource "google_project_iam_member" "cluster_service_account-compute-viewer" {
  project = google_project_iam_member.cluster_service_account-metric_writer.project
  role    = "roles/compute.viewer"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

# Grant access to GCR in the same project that the cluster exists in.
# Make sure workload identity is being used or else this will grant
# tenants on the cluster access to your GCS buckets, and possibly the terraform state.
resource "google_project_iam_member" "cluster_service_account-gcr" {
  project = var.project_id
  role    = "roles/storage.objectViewer"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

# Grant access to modify pub/sub subscriptions
# This permission is used by the image-puller service
resource "google_project_iam_member" "cluster_service_account-pubsub" {
  project = var.project_id
  role    = "roles/pubsub.editor"
  member  = "serviceAccount:${google_service_account.cluster_service_account.email}"
}

# Service account used by CNRM.
resource "google_service_account" "cnrm-system" {
  project      = var.project_id
  account_id   = "cnrm-system"
  display_name = "cnrm-system"
  depends_on   = [google_project_service.iam]
}

# IAM binding to grant CNRM service account access to the project.
resource "google_project_iam_member" "cnrm-owner" {
  project = google_service_account.cnrm-system.project
  role    = "roles/owner"
  member  = "serviceAccount:${google_service_account.cnrm-system.email}"
}

# Service account used by autoneg controller.
resource "google_service_account" "autoneg-system" {
  project      = var.project_id
  account_id   = "autoneg-system"
  display_name = "autoneg-system"
}

resource "google_project_iam_custom_role" "autoneg" {
  project     = var.project_id
  role_id     = "autoneg"
  title       = "AutoNEG Custom Role"
  description = "AutoNEG controller"
  permissions = [
    "compute.backendServices.get",
    "compute.backendServices.update",
    "compute.networkEndpointGroups.use",
    "compute.healthChecks.useReadOnly"
  ]
}

# IAM binding to grant AutoNEG service account access to the project.
resource "google_project_iam_member" "autoneg-system" {
  project = var.project_id
  role    = "projects/${google_project_iam_custom_role.autoneg.project}/roles/${google_project_iam_custom_role.autoneg.role_id}"
  member  = "serviceAccount:${google_service_account.autoneg-system.email}"
}

# Service account used by the user pods.
resource "google_service_account" "user_pod_service_account" {
  project      = var.project_id
  account_id   = "${var.name}-user"
  display_name = "${var.name} user workload"
  depends_on   = [google_project_service.iam]
}

# Grant user service account access to IAP.
resource "google_project_iam_member" "user_pod_service_account-iap-user" {
  project = google_project_iam_member.cluster_service_account-metric_writer.project
  role    = "roles/iap.httpsResourceAccessor"
  member  = "serviceAccount:${google_service_account.user_pod_service_account.email}"
}
