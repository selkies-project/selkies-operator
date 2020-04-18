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

# This script joins the available accelerator types per zone
# with the available regions from GCPing and displays a selection list.

SCRIPT_DIR=$(dirname $(readlink -f $0 2>/dev/null) 2>/dev/null || echo "${PWD}/$(dirname $0)")

source "${SCRIPT_DIR}/gcping.sh"

# Generates a map of zone -> csv of accelerator types.
function getAcceleratorTypes() {
    declare -n result_map="$1"
    declare -a curr_list
    data=$(gcloud compute accelerator-types list --format='csv[no-heading](zone,name)') || return 1

    curr_zone=""
    while read -r line; do
        IFS=',' read -ra toks <<< "$line"
        zone=${toks[0]}
        accelerator=${toks[1]}
        if [[ "${curr_zone}" != "${zone}" ]]; then
            if [[ -n "${curr_zone}" ]]; then
                result_map[$curr_zone]=$(strJoin , ${curr_list[@]})
            fi
            curr_zone=$zone
            curr_list=()
        fi
        curr_list+=($accelerator)
    done <<< $(echo "$data" | sort)
    result_map[$curr_zone]=$(strJoin , ${curr_list[@]})
}

# Helper function to join a string by a delimiter
function strJoin { local IFS="$1"; shift; echo "$*"; }

echo "INFO: Fetching list of accelerator types..." >&2
declare -A ACCELERATOR_MAP
if ! getAcceleratorTypes ACCELERATOR_MAP; then
    echo "ERROR: Failed to obtain list of accelerators" >&2
    exit 1
fi

echo "INFO: Fetching list of GCPing regions..." >&2
declare -A REGION_MAP
if ! parseRegionMap $GCPING_SRC_URL REGION_MAP; then
    echo "ERROR: Failed to parse region map from: ${GCPING_SRC_URL}" >&2
    exit 1
fi

echo "INFO: Measuring ping to regions..." >&2
declare -A RTT_MAP
if ! gcping REGION_MAP RTT_MAP; then
    echo "ERROR: Failed to ping regions" >&2
    exit 1
fi

# Merge data to csv of: ping,region,zone,accelerator
declare -a merged_data
for region in "${!RTT_MAP[@]}"; do
    rtt=${RTT_MAP[$region]}
    declare -a accelerators
    for zone in "${!ACCELERATOR_MAP[@]}"; do
        if [[ $zone =~ $region ]]; then
            IFS="," read -ra accelerators <<< "${ACCELERATOR_MAP[$zone]}"
            if [[ "${#accelerators[@]}" -gt 0 ]]; then
                for i in ${!accelerators[@]}; do
                    merged_data+=("${rtt},${region},${zone},${accelerators[$i]}")
                done
            else
                echo "WARN: region $region has no accelerators" >&2
            fi
        fi
    done
done

# Sort merged data by ping time, decreasing order.
data_sorted=$(for i in "${!merged_data[@]}"; do echo "${merged_data[i]}"; done | sort -t';' -k1 -n -r)

# Add numeric prefix to each data item that user will choose from.
declare -a data_numbered
count=0
for item in $data_sorted; do
    data_numbered+=("${count},${item}")
    ((count+=1))
done

curr_zone=""
for i in ${!data_numbered[@]}; do
    IFS=',' read -ra toks <<< "${data_numbered[$i]}"
    num=${toks[0]}
    rtt=${toks[1]}
    region=${toks[2]}
    zone=${toks[3]}
    accelerator=${toks[4]}
    if [[ "${zone}" != "${curr_zone}" ]]; then
        echo "${zone} (ping ${rtt}ms):" >&2
        curr_zone=$zone
    fi
    echo "  ${num}) ${accelerator}" >&2
done

((count-=1))
read -p "Choose a zone and accelerator (default $count: $zone $accelerator): " INPUT >&2
[[ -z "${INPUT}" ]] && INPUT=$count
IFS=',' read -ra sel <<< "${data_numbered[$INPUT]}"
region=${sel[2]}
zone=${sel[3]}
accel_type=${sel[4]}

echo
echo "  export REGION=${region};"
echo "  export ZONE=${zone};"
echo "  export ACCELERATOR_TYPE=${accel_type};"

echo "exported REGION=${region}" >&2
echo "exported ZONE=${zone}" >&2
echo "exported ACCELERATOR_TYPE=${accel_type}" >&2