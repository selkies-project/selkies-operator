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

variable "project_id" {
  description = "The project ID to host the cluster in"
}

variable "kubernetes_version_prefix" {
  default = "1.14"
}

variable "name" {
  default = "broker"
}

variable "additional_ssl_certificate_domains" {
  description = "list of additional domains to add to the managed certificate."
 type        = list
  default     = []
}

variable "custom_ssl_policy_enabled" {
  description = "Enable custom SSL Policy"
  default     = true
}

variable "custom_ssl_policy_delete" {
  description = "set this in 2-pass ssl_policy removal after running with custom_ssl_policy_enabled = false to remove the ssl_policy resource without dependency issues with the target proxy"
  default     = true
}

variable "ssl_policy_min_tls_version" {
  description = "(optional) - The minimum version of SSL protocol that can be used by the clients\nto establish a connection with the load balancer. Default value: \"TLS_1_0\" Possible values: [\"TLS_1_0\", \"TLS_1_1\", \"TLS_1_2\"]"
  type        = string
  default     = "TLS_1_0"
}

variable "ssl_policy_profile" {
  description = "(optional) - Profile specifies the set of SSL features that can be used by the\nload balancer when negotiating SSL with clients. If using 'CUSTOM',\nthe set of SSL features to enable must be specified in the\n'customFeatures' field.\n\nSee the [official documentation](https://cloud.google.com/compute/docs/load-balancing/ssl-policies#profilefeaturesupport)\nfor information on what cipher suites each profile provides. If\n'CUSTOM' is used, the 'custom_features' attribute **must be set**. Default value: \"COMPATIBLE\" Possible values: [\"COMPATIBLE\", \"MODERN\", \"RESTRICTED\", \"CUSTOM\"]"
  type        = string
  default     = "COMPATIBLE"
}

variable "ssl_policy_custom_features" {
  description = "(optional) - Profile specifies the set of SSL features that can be used by the\nload balancer when negotiating SSL with clients. This can be one of\n'COMPATIBLE', 'MODERN', 'RESTRICTED', or 'CUSTOM'. If using 'CUSTOM',\nthe set of SSL features to enable must be specified in the\n'customFeatures' field.\n\nSee the [official documentation](https://cloud.google.com/compute/docs/load-balancing/ssl-policies#profilefeaturesupport)\nfor which ciphers are available to use. **Note**: this argument\n*must* be present when using the 'CUSTOM' profile. This argument\n*must not* be present when using any other profile."
  type        = set(string)
  default     = null
}

variable "lb_security_policy_enabled" {
  description = "Enable Load Balancer Security Policy. A Security Policy defines a policy that protects load balanced Google Cloud services by permitting traffic only from specified IP ranges or geographical locations"
  type        = bool
  default     = false
}

variable "lb_security_policy_delete" {
  description = "set this in 2-pass security_policy removal after running with lb_security_policy_enabled = false to remove the security_policy resource without dependency issues with the backend service"
  default     = true
}

variable "lb_security_policy_rules" {
  default = [
    {
      action      = "allow"
      priority    = 1000
      expression  = "inIpRange(origin.ip, '0.0.0.0/0')"
      description = "Allow all the traffic"
    }
  ]
}

variable "lb_security_policy_default_rule_action" {
  description = "By default, for each policy you have to configured the default rule that allows/denies all traffic with the lowest priority (2147483647). Possible values allow, deny(403), deny(404), deny(502)"
  default     = "deny(403)"
}