#!/bin/bash

# Copyright 2019 Google Inc.
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

[[ -z "$1" || -z "$2" ]] && echo "USAGE: $0 <user|group|domain> <member> [project id]" && exit 1

[[ ! "$1" =~ user|group|domain ]] && echo "ERROR: invalid member type '$1', must be one of user|group|domain" && exit 1

MEMBER_TYPE=${1,,}
MEMBER=$2

SCRIPT_DIR=$(dirname $(readlink -f $0 2>/dev/null) 2>/dev/null || echo "${PWD}/$(dirname $0)")

# Get project from terraform outputs
PROJECT=${GOOGLE_CLOUD_PROJECT:-${3:-$(cd ${SCRIPT_DIR}/../infra && terraform output project)}};

gcloud projects add-iam-policy-binding ${PROJECT} \
    --member="${MEMBER_TYPE}:${MEMBER}" \
    --role='roles/iap.httpsResourceAccessor' >/dev/null