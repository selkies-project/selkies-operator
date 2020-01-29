#!/bin/bash

# Copyright 2019 Google Inc. All rights reserved.
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

export RED='\033[1;31m'
export CYAN='\033[1;36m'
export GREEN='\033[1;32m'
export NC='\033[0m' # No Color
function log_red() { echo -e "${RED}$@${NC}"; }
function log_cyan() { echo -e "${CYAN}$@${NC}"; }
function log_green() { echo -e "${GREEN}$@${NC}"; }

SCRIPT_DIR=$(dirname $(readlink -f $0 2>/dev/null) 2>/dev/null || echo "${PWD}/$(dirname $0)")

cd "${SCRIPT_DIR}"

PROJECT_ID=$1
INFRA_NAME=$2
CLIENT_ID=$3
CLIENT_SECRET=$4
COOKIE_SECRET=$5
DESTROY=${6,,}

[[ $# -lt 5 ]] && log_red "USAGE: $0 <PROJECT_ID> <INFRA_NAME> <CLIENT_ID> <CLIENT_SECRET> <COOKIE_SECRET> [--destroy]" && exit 1

log_cyan "Creating terraform.auto.tfvars"

cat - | tee terraform.auto.tfvars <<EOF
project_id           = "${PROJECT_ID}"
name                 = "${INFRA_NAME}"
oauth_client_id      = "${CLIENT_ID}"
oauth_client_secret  = "${CLIENT_SECRET}"
broker_cookie_secret = "${COOKIE_SECRET}"
EOF

log_cyan "Creating backend.tf"

cat - | tee backend.tf <<EOF
terraform {
    backend "gcs" {
    bucket  = "${PROJECT_ID}-${INFRA_NAME}-tf-state"
    prefix  = "${INFRA_NAME}"
    }
}
EOF
terraform init -upgrade=true

if [[ "${DESTROY}" == "--destroy" ]]; then
    log_cyan "Running terraform destroy..."
    terraform destroy -auto-approve
else
    log_cyan "Running terraform plan..."
    terraform plan -out terraform.tfplan
    
    log_cyan "Running terraform apply..."
    terraform apply -input=false terraform.tfplan
fi

log_green "Done"