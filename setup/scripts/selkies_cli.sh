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

ACTION=$1
APP=$2
ENDPOINT=$3
[[ $# -lt 1 ]] && echo "USAGE: $0 <start|stop|status|list> [<app name>]" && exit 1
ACTION=${ACTION,,}
APP=${APP,,}

[[ "${ACTION}" != "list" && -z "${APP}" ]] && echo "ERROR: missing app name for action: ${ACTION}" && exit 1

ACCOUNT=${ACCOUNT:-$(gcloud config get-value account)}
[[ -z "${ACCOUNT}" ]] && echo "ERROR: Failed to get gcloud account, did you run 'gcloud auth login'?" && exit 1

case $ACTION in
"list")
    kubectl exec -n pod-broker-system -c pod-broker pod-broker-0 -- \
        curl -s -H "x-goog-authenticated-user-email: ${ACCOUNT}" -XGET localhost:8080/ \
            | jq -r '.apps[].name'
    ;;
"start")
    kubectl exec -n pod-broker-system -c pod-broker pod-broker-0 -- \
        curl -s -H "x-goog-authenticated-user-email: ${ACCOUNT}" -XPOST localhost:8080/${APP}
    ;;
"stop")
    kubectl exec -n pod-broker-system -c pod-broker pod-broker-0 -- \
        curl -s -H "x-goog-authenticated-user-email: ${ACCOUNT}" -XDELETE localhost:8080/${APP} \
            | jq -r .
    ;;
"status")
    kubectl exec -n pod-broker-system -c pod-broker pod-broker-0 -- \
        curl -s -H "x-goog-authenticated-user-email: ${ACCOUNT}" -XGET localhost:8080/${APP} \
            | jq -r .
    ;;
esac