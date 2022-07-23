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
  project = var.project_id
  secret   = "broker-oauth2-client-id"
}

# Access the oauth2 client secret
data "google_secret_manager_secret_version" "oauth2_client_secret" {
  provider = google-beta
  project = var.project_id
  secret   = "broker-oauth2-client-secret"
}

locals {
  cloud_endpoint = "${var.name}.endpoints.${var.project_id}.cloud.goog"
}

resource "null_resource" "cloud-ep-dns" {
  triggers = {
    endpoint = local.cloud_endpoint
    target   = google_compute_global_address.ingress.address
  }

  provisioner "local-exec" {
    command = "${path.module}/create_cloudep.sh"
    environment = {
      NAME    = var.name
      TARGET  = google_compute_global_address.ingress.address
      PROJECT = var.project_id
    }
  }
}

# Managed certificate
resource "google_compute_managed_ssl_certificate" "ingress" {
  provider = google-beta
  project  = var.project_id

  name = "istio-ingressgateway"

  managed {
    domains = ["${local.cloud_endpoint}."]
  }
}

# Firewall rule
resource "google_compute_firewall" "ingress-lb" {
  name    = "istio-ingressgateway-lb"
  project = var.project_id
  network = google_compute_network.broker.self_link

  allow {
    protocol = "tcp"
    ports    = ["8080", "15020"]
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

  security_policy = var.lb_security_policy_enabled ? concat(google_compute_security_policy.policy.*.id, [""])[0] : null
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
  ssl_certificates = concat(tolist([google_compute_managed_ssl_certificate.ingress.self_link]), values(google_compute_managed_ssl_certificate.extras).*.self_link)
  ssl_policy       = var.custom_ssl_policy_enabled ? concat(google_compute_ssl_policy.ssl_policy.*.id, [""])[0] : null
}

# SSL Policy
resource "google_compute_ssl_policy" "ssl_policy" {
  name            = "${var.name}-ssl-policy"
  min_tls_version = var.ssl_policy_min_tls_version
  profile         = var.ssl_policy_profile
  custom_features = var.ssl_policy_profile == "CUSTOM" ? var.ssl_policy_custom_features : null
  count           = var.custom_ssl_policy_delete ? 0 : 1
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

# Load Balancer Security Policy 
resource "google_compute_security_policy" "policy" {
  provider = google-beta
  count    = var.lb_security_policy_delete ? 0 : 1
  name     = "istio-ingressgateway-policy"

  dynamic "rule" {
    for_each = var.lb_security_policy_rules
    content {
      action   = rule.value["action"]
      priority = rule.value["priority"]
      match {
        expr {
          expression = rule.value["expression"]
        }
      }
      description = rule.value["description"]
    }
  }

  # By default, for each policy you start with one rule that allows/denies all traffic with the lowest priority (2,147,483,647)
  rule {
    action   = var.lb_security_policy_default_rule_action
    priority = "2147483647"
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }
    description = "default rule"
  }
}
