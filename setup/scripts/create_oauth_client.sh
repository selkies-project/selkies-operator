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

# Font colors
export RED='\033[1;31m'
export CYAN='\033[1;36m'
export GREEN='\033[1;32m'
export NC='\033[0m' # No Color

export GCLOUD=${GCLOUD:-"gcloud -q"}

function log_cyan() {
    echo -e "${CYAN}$@${NC}" >&2
}

APP_NAME=$1

[[ -z "${APP_NAME}" ]] && echo "USAGE: $0 <app name>" && exit 1

# Ensure that the API is enabled.
$GCLOUD services enable iap.googleapis.com >/dev/null

# Helper function that retries failed alpha commands up to 5 times.
function gcloud_alpha() {
    local count=0
    while [[ $count -lt 5 ]]; do
        gcloud -q alpha $@
        [[ $? -eq 0 ]] && return 0
        ((count=count+1))
        sleep 2
    done
    return 1
}

# Check to see if OAuth brand already exists
log_cyan "INFO: Creating OAuth Brand"
BRAND_ID=$(gcloud_alpha iap oauth-brands list --format='value(name)' --limit=1)
if [[ -z "${BRAND_ID}" ]]; then
    # Create the OAuth Brand
    BRAND_ID=$(gcloud_alpha iap oauth-brands create --application_title="${APP_NAME?}" --support_email=$($GCLOUD config get-value account) --format='value(name)')
    [[ $? -ne 0 ]] && echo "ERROR: Failed to create oauth brand." && exit 1
    sleep 2
else
    log_cyan "INFO:   Using existing brand: ${BRAND_ID}"
fi

# Check to see if OAuth client already exists.
CLIENT_ID=
CLIENT_SECRET=
IFS=',' read -ra toks < <(gcloud_alpha iap oauth-clients list ${BRAND_ID?} --filter="displayName~'${APP_NAME?}'"  --limit=1 --format 'csv[no-heading](name,secret)') 
if [[ ${#toks[@]} -eq 0 ]]; then
    log_cyan "INFO: Creating OAuth client"
    IFS=',' read -ra toks < <(gcloud_alpha iap oauth-clients create ${BRAND_ID?} --display_name="${APP_NAME?}" --format 'csv[no-heading](name,secret)')
    [[ $? -ne 0 ]] && echo "ERROR: Failed to create oauth client." && exit 1
    CLIENT_ID=$(basename ${toks[0]})
    CLIENT_SECRET=${toks[1]}
else
    log_cyan "INFO:   Using existing client: ${toks[0]}"
    CLIENT_ID=$(basename ${toks[0]})
    CLIENT_SECRET=${toks[1]}
fi

[[ -z "${CLIENT_ID}" || -z "${CLIENT_SECRET}" ]] && echo "ERROR: Failed to get oauth client" && exit 1

echo "export CLIENT_ID=${CLIENT_ID}"
echo "export CLIENT_SECRET=${CLIENT_SECRET}"

log_cyan "INFO: exported CLIENT_ID"
log_cyan "INFO: exported CLIENT_SECRET"
log_cyan "INFO: Done"
