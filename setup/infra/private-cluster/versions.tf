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

terraform {
  backend "gcs" {}
  required_version = ">= 1.2.0"
  required_providers {
    external    = "~> 1.2.0"
    google      = "~> 4.25.0, <4.25.6"
    google-beta ="~> 4.25.0"
    kubernetes  = "~> 2.11.0"
    template    = "~> 2.1"
    null        = "~> 2.1"
    random      = "~> 2.2"
  }
}
