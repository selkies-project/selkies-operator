#!/bin/bash

# Copyright 2021 The Selkies Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

NODE_POD_CIDR=$(kubectl get node -o jsonpath='{.items[0].spec.podCIDR}')
[[ -z "${NODE_POD_CIDR}" ]] && echo "ERROR: failed to get node pod cidr" && exit 1

IFS="." read -ra octets <<< ${NODE_POD_CIDR}

#if [[ "${octets[1]}" -lt 32 ]]; then
#    echo "INFO: Pod CIDR range (${NODE_POD_CIDR}) fits within the RFC1918 range and does not need fixing."
#    exit 0
#fi

POD_CIDR="${octets[0]}.${octets[1]}.0.0/18"

set +e
read -r -d '' MASQ_CONFIG <<EOF
{
    "nonMasqueradeCIDRs": [
        "${POD_CIDR}"
    ],
    "resyncInterval": "60s"
}
EOF
set -e

CURR_CONFIG_MAP=$(kubectl get configmap -n kube-system ip-masq-agent -o json 2>/dev/null || true)
if [[ -n "${CURR_CONFIG_MAP}" ]]; then
    CURR_CONFIG=$(echo "$CURR_CONFIG_MAP" | jq -r '.data.config|fromjson')
    echo "INFO: Merging ip-masq-agent config with existing nonMasqueradeCIDRs"
    MASQ_CONFIG=$(echo ${CURR_CONFIG} | jq --arg cidr "${POD_CIDR}" -r '.nonMasqueradeCIDRs=(.nonMasqueradeCIDRs+[$cidr]|unique)')
else
    echo "INFO: Creating ip-masq-agent config for Pod CIDR range: ${POD_CIDR}"
fi

kubectl create configmap ip-masq-agent -n kube-system --from-file config=<(echo "${MASQ_CONFIG}") --dry-run -o yaml | \
    kubectl apply -f -
