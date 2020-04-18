#!/bin/bash

# Google LLC 2019
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

NAMESPACE=$1
SERVICE_ACCOUNT=$2

[[ $# -lt 2 ]] && echo "USAGE: $0 <NAMESPACE> <K8S SERVICE ACCOUNT> ..." && exit 1

# From: https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity
kubectl run --rm -it \
    --generator=run-pod/v1 \
    --image google/cloud-sdk:slim \
    --namespace ${NAMESPACE} \
    --serviceaccount ${SERVICE_ACCOUNT} \
    workload-identity-test
