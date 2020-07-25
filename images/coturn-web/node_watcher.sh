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
    kubectl get node -l "cloud.google.com/gke-nodepool=turn" -o json | jq -c -r '[.items[].status.addresses[] | select(.type == "ExternalIP") | {ip: .address}]' \
        > /tmp/node_ips.json
    
    # Inject IPs into template and apply to cluster.
    cat /tmp/endpoints-template.json | jq --slurpfile nodeIPs /tmp/node_ips.json '.subsets[].addresses=$nodeIPs[]' | \
        kubectl apply -f - > /dev/null

    sleep 10
done
