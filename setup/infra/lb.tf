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

# Static IP for the broker
resource "google_compute_global_address" "ingress" {
  project = var.project_id
  name    = var.name
}

# Access the oauth2 client id
data "google_secret_manager_secret_version" "oauth2_client_id" {
  provider = google-beta
  secret = "broker-oauth2-client-id"
}

# Access the oauth2 client secret
data "google_secret_manager_secret_version" "oauth2_client_secret" {
  provider = google-beta
  secret = "broker-oauth2-client-secret"
}

# Cloud endpoints for DNS
module "cloud-ep-dns" {
  # Return to module registry after this is merged: https://github.com/terraform-google-modules/terraform-google-endpoints-dns/pull/2
  #source      = "terraform-google-modules/endpoints-dns/google"
  source      = "github.com/danisla/terraform-google-endpoints-dns?ref=0.12upgrade"
  project     = var.project_id
  name        = var.name
  external_ip = google_compute_global_address.ingress.address
}

# Managed certificate
resource "google_compute_managed_ssl_certificate" "ingress" {
  provider = google-beta
  project  = var.project_id

  name = "istio-ingressgateway"

  managed {
    domains = ["${module.cloud-ep-dns.endpoint}."]
  }
}

# Firewall rule
resource "google_compute_firewall" "ingress-lb" {
  name    = "istio-ingressgateway-lb"
  project = var.project_id
  network = google_compute_network.broker.self_link

  allow {
    protocol = "tcp"
  }

  source_ranges = [
    "130.211.0.0/22",
    "35.191.0.0/16"
  ]
}

# Health check
resource "google_compute_health_check" "ingress" {
  project            = var.project_id
  name               = "istio-ingressgateway"
  check_interval_sec = 10

  tcp_health_check {
    port = "15020"
  }
}

# BackendService
resource "google_compute_backend_service" "ingress" {
  project       = var.project_id
  name          = "istio-ingressgateway"
  health_checks = [google_compute_health_check.ingress.self_link]
  protocol      = "HTTP"
  timeout_sec   = 86400
  iap {
    oauth2_client_id     = data.google_secret_manager_secret_version.oauth2_client_id.secret_data
    oauth2_client_secret = data.google_secret_manager_secret_version.oauth2_client_secret.secret_data
  }
  lifecycle {
    ignore_changes = [
      backend
    ]
  }
}

# URL map - HTTPS
resource "google_compute_url_map" "ingress" {
  project         = var.project_id
  name            = "istio-ingressgateway"
  default_service = google_compute_backend_service.ingress.self_link
}

# Target HTTP proxy
resource "google_compute_target_http_proxy" "ingress" {
  project = var.project_id
  name    = "istio-ingressgateway"
  url_map = google_compute_url_map.ingress.self_link
}

# Target HTTPS proxy
resource "google_compute_target_https_proxy" "ingress" {
  project          = var.project_id
  name             = "istio-ingressgateway"
  url_map          = google_compute_url_map.ingress.self_link
  ssl_certificates = concat(list(google_compute_managed_ssl_certificate.ingress.self_link), values(google_compute_managed_ssl_certificate.extras).*.self_link)
}

# Forwarding rule - HTTP
resource "google_compute_global_forwarding_rule" "ingress-http" {
  project = var.project_id

  name       = "istio-ingressgateway-http"
  ip_address = google_compute_global_address.ingress.address
  target     = google_compute_target_http_proxy.ingress.self_link
  port_range = "80"
}


# Forwarding rule - HTTPS
resource "google_compute_global_forwarding_rule" "ingress" {
  project = var.project_id

  name       = "istio-ingressgateway"
  ip_address = google_compute_global_address.ingress.address
  target     = google_compute_target_https_proxy.ingress.self_link
  port_range = "443"
}

# Certificates for additional domains
resource "google_compute_managed_ssl_certificate" "extras" {
  for_each = toset(var.additional_ssl_certificate_domains)
  provider = google-beta
  project  = var.project_id

  name = element(split(".", each.value), 0)

  managed {
    domains = [each.value]
  }
}
