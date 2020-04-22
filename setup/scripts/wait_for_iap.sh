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

SA_EMAIL=$1
CLIENT_ID=$2
ENDPOINT=$3

[[ -z "${SA_EMAIL}" || -z "${CLIENT_ID}" || -z "${ENDPOINT}" ]] && echo "USAGE: $0 <sa email> <client id> <endpoint>" && exit 1

TOKEN=$(curl -f -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" | jq -r '.access_token')

ID_TOKEN=$(curl -f -s -H "Authorization: Bearer ${TOKEN?}" --header 'Content-Type: application/json' -d '{  "audience": "'${CLIENT_ID?}'", "includeEmail": "true" }' "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/${SA_EMAIL?}:generateIdToken" | jq -r '.token')

[[ -z "${ID_TOKEN}" ]] && echo "ERROR: Failed to obtain ID token" && exit 1

echo "INFO: Waiting for: ${ENDPOINT}"
count=0
while [[ "${count}" -le 5 ]]; do
    STATUS=$(curl --connect-timeout 1 -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer ${ID_TOKEN?}" "${ENDPOINT?}")
    [[ "$STATUS" -eq 200 ]] && ((count=count+1))
    [[ "$STATUS" -eq 403 ]] && echo "ERROR: failed to get valid IAP credentials." && exit 1
    [[ "$STATUS" -eq 302 ]] && echo "ERROR: unexpected 302 from IAP endpoint, possible invalid ID token." && exit 1
    sleep 2
done
echo "INFO: Done"