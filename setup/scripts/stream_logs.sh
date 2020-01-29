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

PROJECT=$1
shift || true

[[ -z "${PROJECT}" ]] && echo "USAGE: $0 <PROJECT>" && exit 1

SCRIPT_DIR=$(dirname $(readlink -f $0 2>/dev/null) 2>/dev/null || echo "${PWD}/$(dirname $0)")

BUILD_ID=$(gcloud builds list --project ${PROJECT} --sort-by=startTime --limit=1 --format='value(id)' | cut -f1)

gcloud builds log --stream --project ${PROJECT} ${BUILD_ID} $@