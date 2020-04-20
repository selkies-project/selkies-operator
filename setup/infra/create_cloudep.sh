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

cleanup() {
    rm -f cloudep.yaml
}
trap cleanup EXIT

function log() {
    level=$1
    msg=$2
    echo "${level}: $msg" >&2
}

[[ -z "$NAME" || -z "$TARGET" || -z "$PROJECT" ]] && echo "ERRO: required env vars: NAME TARGET PROJECT" && exit 1

SVC="${NAME}.endpoints.${PROJECT}.cloud.goog"

cat - > cloudep.yaml <<EOF
swagger: "2.0"
info:
  description: "Cloud Endpoints DNS"
  title: "Cloud Endpoints DNS"
  version: "1.0.0"
paths: {}
host: "${SVC}"
x-google-endpoints:
- name: "${SVC}"
  target: "${TARGET}"
EOF

log "INFO" "Enabling Service Management API"

gcloud --project ${PROJECT} services enable servicemanagement.googleapis.com serviceusage.googleapis.com >&2

log "INFO" "Forcing undelete of Endpoint Service"
gcloud --project ${PROJECT} endpoints services undelete ${SVC} >/dev/null 2>&1 || true

gcloud --project ${PROJECT} -q endpoints services deploy cloudep.yaml 1>&2

log "INFO" "Cloud Endpoint config ID ${PROJECT}:"
gcloud --project ${PROJECT} endpoints services describe ${SVC} --format='value(serviceConfig.id)'

log "INFO" "Created Cloud Endpoint: ${SVC}"