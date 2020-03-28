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

variable project_id {}

data "terraform_remote_state" "broker" {
  backend = "gcs"

  config = {
    bucket = "${var.project_id}-broker-tf-state"
    prefix = "broker"
  }

  workspace = "default"
}

# Workload Identity IAM binding for CNRM.
resource "google_service_account_iam_member" "cnrm-sa-workload-identity" {
  service_account_id = data.terraform_remote_state.broker.outputs.cnrm-system-service-account
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[cnrm-system/cnrm-controller-manager]"
}

# Workload Identity IAM binding for broker in default namespace.
resource "google_service_account_iam_member" "broker-default-sa-workload-identity" {
  service_account_id = data.terraform_remote_state.broker.outputs.node-service-account
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[default/pod-broker]"
}

# Workload Identity IAM binding for AutoNEG controller.
resource "google_service_account_iam_member" "autoneg-sa-workload-identity" {
  service_account_id = data.terraform_remote_state.broker.outputs.autoneg-system-service-account
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[autoneg-system/autoneg-system]"
}