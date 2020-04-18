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

# config file from gcping containing region map.
GCPING_SRC_URL="https://raw.githubusercontent.com/ImJasonH/gcping/master/config.js"

function usage() {
    echo "USAGE: $0 <nearest|furthest>" >&2
}

function parseRegionMap() {
    src_url=$1
    declare -n region_map="$2"

    data=$(curl -sf ${src_url}) || return 1

    while read -r line; do
        [[ ! "${line}" =~ .*http.* || "${line}" =~ global ]] && continue
        IFS=' ' read -ra toks <<< "$line"
        region=${toks[0]//[\":]/}
        region_url=${toks[1]//[\",]/}
        region_map[${region}]=${region_url}
    done <<< "$data"
}

function gcping() {
    declare -n region_map="$1"
    declare -n rtt_map="$2"

    for region in "${!region_map[@]}"; do
        url=${region_map[$region]}
	rtt_ms=30000
        rtt=$(curl --connect-timeout 2 -sf -o /dev/null -w "%{time_starttransfer}\n" ${url})
        [[ $? -eq 0 ]] && rtt_ms=$(awk -vp=${rtt} 'BEGIN{printf "%d" ,p * 1000}')
        rtt_map[$region]=${rtt_ms}
        echo "PING ${region}: ${rtt_ms}ms" >&2
    done
}

function findNearestRegion() {
    declare -n rtt_map="$1"

    local nearest_rtt=100000
    local nearest_region=""

    for region in "${!rtt_map[@]}"; do
        rtt=${rtt_map[$region]}
        if [[ "${rtt}" -lt "${nearest_rtt}" ]]; then
            nearest_rtt=${rtt}
            nearest_region=$region
        fi
    done
    echo "${nearest_region}"
}

function findFurthestRegion() {
    declare -n rtt_map="$1"

    local furthest_rtt=0
    local furthest_region=""

    for region in "${!rtt_map[@]}"; do
        rtt=${rtt_map[$region]}
        if [[ "${rtt}" -gt "${furthest_rtt}" ]]; then
            furthest_rtt=${rtt}
            furthest_region=$region
        fi
    done
    echo "${furthest_region}"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    [[ -z "${1}" ]] && usage && exit 1

    declare -A REGION_MAP
    if ! parseRegionMap $GCPING_SRC_URL REGION_MAP; then
        echo "ERROR: Failed to parse region map from: ${GCPING_SRC_URL}" >&2
        exit 1
    fi

    declare -A RTT_MAP
    if ! gcping REGION_MAP RTT_MAP; then
        echo "ERROR: Failed to ping regions" >&2
        exit 1
    fi

    case ${1,,} in
        "nearest") findNearestRegion RTT_MAP ;;
        "furthest") findFurthestRegion RTT_MAP ;;
        *) usage && exit 1 ;;
    esac
fi

