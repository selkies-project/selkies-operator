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

# Grant access to GCR in the same project that the cluster exists in.
# Make sure workload identity is being used or else this will grant
# tenants on the cluster access to your GCS buckets, and possibly the terraform state.
resource "google_project_iam_member" "cluster_service_account-gcr" {
  project = var.project_id
  role    = "roles/storage.objectViewer"
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

# Workload Identity IAM binding for CNRM.
resource "google_service_account_iam_member" "cnrm-sa-workload-identity" {
  service_account_id = google_service_account.cnrm-system.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[cnrm-system/cnrm-controller-manager]"
  depends_on = [
    module.broker-west
  ]
}

# Workload Identity IAM binding for broker in default namespace.
resource "google_service_account_iam_member" "broker-default-sa-workload-identity" {
  service_account_id = google_service_account.cluster_service_account.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/pod-broker]"
}
