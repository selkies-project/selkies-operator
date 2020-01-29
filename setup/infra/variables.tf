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

variable "project_id" {
  description = "The project ID to host the cluster in"
}

variable "kubernetes_version_prefix" {
  # Issues blocking 1.15:
  #  workload identity timeouts: https://b.corp.google.com/issues/146622472
  default = "1.14.8"
}

variable "name" {
  default = "broker"
}

variable "broker_cookie_secret" {
  description = "Secret used to create broker cookie added to pod-broker secret"
}

variable "oauth_client_id" {
  description = "oauth client id for IAP added to iap-oauth k8s secret"
}

variable "oauth_client_secret" {
  description = "oauth client secret for IAP added to iap-oauth k8s secret"
}