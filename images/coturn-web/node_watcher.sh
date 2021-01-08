#!/bin/bash

# Copyright 2019 Google LLC
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

# Template endpoints JSON.
# The subsets.addresses array is populated with the current list of node ips: {ip: NODE_IP}
cat - > /tmp/endpoints-template.json <<EOF
{
    "apiVersion": "v1",
    "kind": "Endpoints",
    "metadata": {
        "name": "${DISCOVERY_SVC_NAME?}",
        "namespace": "${NAMESPACE?}"
    },
    "subsets": [
        {
            "addresses": [],
            "ports": [
                {
                    "name": "${SVC_PORT_NAME}",
                    "port": ${SVC_PORT?},
                    "protocol": "TCP"
                }
            ]
        }
    ]
}
EOF

echo "INFO: Starting TURN node watcher"

while true; do
    # Save all node IPs to JSON file.
    PRIVATE_CLUSTER=$(kubectl get node $NODE_NAME -o jsonpath='{.metadata.labels.cloud\.google\.com/gke-private-cluster}')
    if [[ "${PRIVATE_CLUSTER}" == "true" ]]; then
        TOKEN=$(curl -s -H "Metadata-Flavor: Google" "http://metadata/computeMetadata/v1/instance/service-accounts/default/token" | jq -r '.access_token')
        ZONE=$(curl -s -H "Metadata-Flavor: Google" "http://metadata/computeMetadata/v1/instance/zone")
        curl -s -H "Authorization: Bearer $TOKEN" -H "Metadata-Flavor: Google" "https://compute.googleapis.com/compute/v1/${ZONE}/instances" | \
        jq -c -r '[.items[] | select(.tags.items | index("gke-turn")) | .networkInterfaces[] | .accessConfigs[] | {ip: .natIP}]' \
            > /tmp/node_ips.json 
    else
        kubectl get node -l "cloud.google.com/gke-nodepool=turn" -o json | jq -c -r '[.items[].status.addresses[] | select(.type == "ExternalIP") | {ip: .address}]' \
            > /tmp/node_ips.json
    fi
    
    if [[ $(jq '.|length' /tmp/node_ips.json) -eq 0 ]]; then
        echo "WARN: No nodes found"
    else
        # Inject IPs into template and apply to cluster.
        cat /tmp/endpoints-template.json | jq --slurpfile nodeIPs /tmp/node_ips.json '.subsets[].addresses=$nodeIPs[]' | \
            kubectl apply --overwrite=true -f - |  > /dev/null
    fi

    sleep 10
done
