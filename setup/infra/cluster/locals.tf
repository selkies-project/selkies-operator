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

// Built from:
//   gcloud compute accelerator-types list |grep nvidia-tesla- | grep -v vws | sort -k2 -r
locals {
  // Choose 2 regions that both contain the desired accelerator type
  cluster_node_zones = {
    "us-west1"                = ["us-west1-a", "us-west1-b"],
    "us-west2"                = ["us-west2-b", "us-west2-c"],
    "us-central1"             = ["us-central1-a", "us-central1-b"],
    "us-east1"                = ["us-east1-c", "us-east1-d"],
    "us-east4"                = ["us-east4-a", "us-east4-b"],
    "northamerica-northeast1" = ["northamerica-northeast1-a", "northamerica-northeast1-b"],
    "southamerica-east1"      = ["southamerica-east1-c"],
    "europe-west1"            = ["europe-west1-b", "europe-west1-d"],
    "europe-west2"            = ["europe-west2-a", "europe-west2-b"],
    "europe-west4"            = ["europe-west4-b", "europe-west4-c"],
    "asia-east1"              = ["asia-east1-a", "asia-east1-c"],
    "asia-northeast1"         = ["asia-northeast1-a", "asia-northeast1-c"],
    "asia-northeast3"         = ["asia-northeast3-b", "asia-northeast3-c"],
    "asia-south1"             = ["asia-south1-a", "asia-south1-b"],
    "asia-southeast1"         = ["asia-southeast1-b", "asia-southeast1-c"],
    "australia-southeast1"    = ["australia-southeast1-a", "australia-southeast1-b"],
  }

  accelerator_type_regions = {
    "us-west1"                = "nvidia-tesla-t4",
    "us-west2"                = "nvidia-tesla-p4",
    "us-central1"             = "nvidia-tesla-t4",
    "us-east1"                = "nvidia-tesla-t4",
    "us-east4"                = "nvidia-tesla-p4",
    "northamerica-northeast1" = "nvidia-tesla-p4",
    "southamerica-east1"      = "nvidia-tesla-t4",
    "europe-west1"            = "nvidia-tesla-p100",
    "europe-west2"            = "nvidia-tesla-t4",
    "europe-west4"            = "nvidia-tesla-t4",
    "asia-east1"              = "nvidia-tesla-p100",
    "asia-northeast1"         = "nvidia-tesla-t4",
    "asia-northeast3"         = "nvidia-tesla-t4",
    "asia-south1"             = "nvidia-tesla-t4",
    "asia-southeast1"         = "nvidia-tesla-t4",
    "australia-southeast1"    = "nvidia-tesla-p4",
  }
}