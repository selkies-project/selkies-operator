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
  cluster_regions = {
    "us-west1"                = 0,  # The Dalles, Oregon, USA
    "us-west2"                = 1,  # Los Angeles, California, USA
    "us-central1"             = 2,  # Council Bluffs, Iowa, USA
    "us-east1"                = 3,  # Moncks Corner, South Carolina, USA  
    "us-east4"                = 4,  # Ashburn, Northern Virginia, USA
    "northamerica-northeast1" = 5,  # Montréal, Québec, Canada
    "southamerica-east1"      = 6,  # Osasco (São Paulo), Brazil
    "europe-west1"            = 7,  # St. Ghislain, Belgium
    "europe-west2"            = 8,  # London, England, UK
    "europe-west4"            = 9,  # Eemshaven, Netherlands
    "asia-east1"              = 10, # Changhua County, Taiwan
    "asia-northeast1"         = 11, # Tokyo, Japan
    "asia-northeast3"         = 12, # Seoul, South Korea
    "asia-south1"             = 13, # Mumbai, India
    "asia-southeast1"         = 14, # Jurong West, Singapore
    "australia-southeast1"    = 15, # Sydney, Australia
  }
}