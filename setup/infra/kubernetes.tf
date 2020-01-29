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

resource "kubernetes_namespace" "istio-system" {
  provider = kubernetes.us-west1
  metadata {
    name = "istio-system"
  }

  lifecycle {
    ignore_changes = [metadata]
  }
}

resource "kubernetes_secret" "iap-oauth" {
  provider = kubernetes.us-west1
  metadata {
    name      = "iap-oauth"
    namespace = "istio-system"
  }

  data = {
    client_id     = var.oauth_client_id
    client_secret = var.oauth_client_secret
  }

  type = "Opaque"

  depends_on = [kubernetes_namespace.istio-system]
}

resource "kubernetes_secret" "pod-broker" {
  provider = kubernetes.us-west1
  metadata {
    name      = "pod-broker"
    namespace = "default"
  }

  data = {
    COOKIE_SECRET = var.broker_cookie_secret
  }

  type = "Opaque"
}