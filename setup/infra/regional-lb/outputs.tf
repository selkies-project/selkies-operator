/**
 * Copyright 2021 The Selkies Authors
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

output cloud-ep-endpoint {
  description = "Cloud Endpoint DNS"
  value       = local.cloud_endpoint
}

output backend-service {
  description = "name of backend service that NEGs should attach to"
  value       = google_compute_backend_service.ingress.name
}