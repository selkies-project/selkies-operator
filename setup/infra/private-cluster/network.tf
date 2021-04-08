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

data "google_compute_network" "broker" {
  name = var.name
}

resource "google_compute_subnetwork" "broker" {
  name                     = "${var.name}-${var.region}"
  ip_cidr_range            = var.ip_cidr_range.nodes != "" ?  var.ip_cidr_range.nodes : "10.${2 + lookup(local.cluster_regions, var.region)}.0.0/16"
  region                   = var.region
  network                  = data.google_compute_network.broker.self_link
  private_ip_google_access = true

  secondary_ip_range = [
    {
      range_name    = "${var.region}-pods"
      ip_cidr_range = var.ip_cidr_range.pods != "" ?  var.ip_cidr_range.pods : "172.${16 + lookup(local.cluster_regions, var.region)}.0.0/18"
    },
    {
      range_name    = "${var.region}-services"
      ip_cidr_range = var.ip_cidr_range.services != "" ?  var.ip_cidr_range.services : "192.168.${lookup(local.cluster_regions, var.region)}.0/24"
    },
  ]
}

resource "google_compute_router" "router-nat" {
  provider = google-beta
  name     = "broker-nat-${var.region}"
  network  = data.google_compute_network.broker.id
  region   = var.region

  bgp {
    asn = var.router_asn
  }
}

module "cloud-nat" {
  source     = "terraform-google-modules/cloud-nat/google"
  version    = "~> 1.2"
  project_id = var.project_id
  region     = var.region
  router     = google_compute_router.router-nat.name
}