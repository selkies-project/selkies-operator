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

resource "google_compute_network" "broker" {
  name                    = var.name
  auto_create_subnetworks = false
  depends_on = [
    google_project_service.compute
  ]
}

resource "google_compute_firewall" "turn" {
  name = "k8s-fw-gke-turn"
  network = replace(
    google_compute_network.broker.self_link,
    "https://www.googleapis.com/compute/v1/",
    "",
  )

  allow {
    protocol = "tcp"
    ports    = ["3478", "25000-25100"]
  }

  allow {
    protocol = "udp"
    ports    = ["3478", "25000-25100"]
  }

  target_tags   = ["gke-turn"]
  source_ranges = ["0.0.0.0/0"]
}
