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

[[ $# -lt 2 ]] && echo "USAGE: $0 <INFRA NAME> <REGION> <REGION> ..." && exit 1

INFRA_NAME=$1
shift
REGIONS=$@

# Wait for all clusters to become RUNNING
for REGION in $REGIONS; do
    echo "Waiting for cluster in '${REGION}'..."
    until [[ $(gcloud container clusters describe ${INFRA_NAME}-${REGION} --region ${REGION} --format="value(status)" 2>/dev/null) == "RUNNING" ]]; do
        sleep 2
    done
    echo "Cluster in '${REGION}' is ready."
done

echo "Done. All clusters ready."