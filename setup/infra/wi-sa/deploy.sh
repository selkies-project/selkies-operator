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

export RED='\033[1;31m'
export CYAN='\033[1;36m'
export GREEN='\033[1;32m'
export NC='\033[0m' # No Color
function log_red() { echo -e "${RED}$@${NC}"; }
function log_cyan() { echo -e "${CYAN}$@${NC}"; }
function log_green() { echo -e "${GREEN}$@${NC}"; }

SCRIPT_DIR=$(dirname $(readlink -f $0 2>/dev/null) 2>/dev/null || echo "${PWD}/$(dirname $0)")

cd "${SCRIPT_DIR}"

# Fetch any Secret Manager secrets named broker-tfvars* and same them to .auto.tfvars files.
for secret in $(gcloud -q secrets list --filter=name~broker-tfvars- --format="value(name)"); do
    latest=$(gcloud secrets versions list ${secret} --sort-by=created --format='value(name)' --limit=1)
    dest="${secret/broker-tfvars-/}.auto.tfvars"
    log_cyan "Creating ${dest} from secret: ${secret}"
    gcloud -q secrets versions access ${latest} --secret ${secret} > ${dest}
done

export TF_IN_AUTOMATION=1

# Set default project for google provider.
export GOOGLE_PROJECT=${TF_VAR_project_id?}

# Initialize backend and select workspace
terraform init -upgrade=true -input=false \
    -backend-config="bucket=${TF_VAR_project_id?}-${TF_VAR_name?}-tf-state" \
    -backend-config="prefix=${TF_VAR_name?}" || true
terraform workspace select ${TERRAFORM_WORKSPACE_NAME?} || terraform workspace new ${TERRAFORM_WORKSPACE_NAME?}
terraform init -input=false \
    -backend-config="bucket=${TF_VAR_project_id?}-${TF_VAR_name?}-tf-state" \
    -backend-config="prefix=${TF_VAR_name?}" || true

if [[ "${ACTION?}" == "destroy" ]]; then
    log_cyan "Running terraform destroy..."
    terraform destroy -auto-approve -input=false 
elif [[ "${ACTION?}" == "plan" ]]; then
    log_cyan "Running terraform plan..."
    terraform plan -out terraform.tfplan -input=false
elif [[ "${ACTION?}" == "apply" ]]; then
    log_cyan "Running terraform plan..."
    terraform plan -out terraform.tfplan -input=false

    log_cyan "Running terraform apply..."
    terraform apply -input=false terraform.tfplan
fi

log_green "Done"