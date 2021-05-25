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

function usage() {
    echo "USAGE: $0 <start|stop|status|list> [<app name>] [-u <user>] [--ctx <kube context>]" && exit 1
}
[[ $# -eq 0 ]] && usage && exit 1

ACCOUNT=""
CTX=""
while (( "$#" )); do
    case ${1} in
        start|stop|status)
            ACTION=${1}
            shift
            APP=${1}
            ;;
        list)
            ACTION=$1
            ;;
        "-u")
            shift
            ACCOUNT=$1
            ;;
        "--ctx")
            shift
            CTX="--context $1"
            ;;
        *)  "ERROR: Invalid argument '$1', USAGE: pod-broker <build|push|deploy-REGION> [-p <project id>]" && return 1 ;;
    esac
    shift
done

[[ "${ACTION}" != "list" && -z "${APP}" ]] && echo "ERROR: missing app name for action: ${ACTION}" && exit 1

if [[ -z "${ACCOUNT}" ]]; then
    ACCOUNT=$(gcloud config get-value account)
    [[ -z "${ACCOUNT}" ]] && echo "ERROR: Failed to get gcloud account, did you run 'gcloud auth login'?" && exit 1
fi

AUTH_HEADER=$(kubectl $CTX get cm -n pod-broker-system pod-broker-config -o jsonpath='{.data.POD_BROKER_PARAM_AuthHeader}')
POD=$(kubectl $CTX get pod -n pod-broker-system -l app=pod-broker -o jsonpath='{..metadata.name}')
[[ -z "${POD}" ]] && echo "ERROR: failed to get pod-broker pod from cluster" && exit 1

case $ACTION in
"list")
    kubectl $CTX exec -n pod-broker-system -c pod-broker ${POD} -- \
        curl -s -H "${AUTH_HEADER}: ${ACCOUNT}" -XGET localhost:8080/ \
            | jq -r '.apps[].name'
    ;;
"start")
    kubectl $CTX exec -n pod-broker-system -c pod-broker ${POD} -- \
        curl -s -H "${AUTH_HEADER}: ${ACCOUNT}" -XPOST localhost:8080/${APP}
    ;;
"stop")
    kubectl $CTX exec -n pod-broker-system -c pod-broker ${POD} -- \
        curl -s -H "${AUTH_HEADER}: ${ACCOUNT}" -XDELETE localhost:8080/${APP} \
            | jq -r .
    ;;
"status")
    kubectl $CTX exec -n pod-broker-system -c pod-broker ${POD} -- \
        curl -s -H "${AUTH_HEADER}: ${ACCOUNT}" -XGET localhost:8080/${APP} \
            | jq -r .
    ;;
esac