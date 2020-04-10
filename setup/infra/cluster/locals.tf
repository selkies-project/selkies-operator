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
  // NOTE: if you plan on using accelerators, choose regions that contain the desired accelerator type
  cluster_node_zones = {
    "us-west1"                = ["us-west1-a"],
    "us-west2"                = ["us-west2-b"],
    "us-central1"             = ["us-central1-a"],
    "us-east1"                = ["us-east1-c"],
    "us-east4"                = ["us-east4-a"],
    "northamerica-northeast1" = ["northamerica-northeast1-a"],
    "southamerica-east1"      = ["southamerica-east1-c"],
    "europe-west1"            = ["europe-west1-b"],
    "europe-west2"            = ["europe-west2-a"],
    "europe-west4"            = ["europe-west4-b"],
    "asia-east1"              = ["asia-east1-a"],
    "asia-northeast1"         = ["asia-northeast1-a"],
    "asia-northeast3"         = ["asia-northeast3-b"],
    "asia-south1"             = ["asia-south1-a"],
    "asia-southeast1"         = ["asia-southeast1-b"],
    "australia-southeast1"    = ["australia-southeast1-a"],
  }
}