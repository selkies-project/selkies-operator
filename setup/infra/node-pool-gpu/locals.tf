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