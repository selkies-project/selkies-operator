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

NODE_POOL=$1
MIN_NODES=$2
MAX_NODES=$3
CRON_SCHEDULE_UP=$4
CRON_SCHEDULE_DOWN=$5
REGION=$6

[[ $# -ne 6 ]] && echo "USAGE: $0 <node pool name> <min nodes> <max nodes> <cron schedule up> <cron schedule down> <region>" && exit 1

[[ ${MIN_NODES} -lt 1 ]] && echo "ERROR: min nodes must be greater than 0. This is the min node pool size when scaling UP." && exit 1

IFS=' ' read -ra toks < <(echo "$CRON_SCHEDULE_UP")
[[ ${#toks[@]} -ne 5 ]] && echo "ERROR: Invalid CRON_SCHEDULE_UP, cron schedule must be 5 elements, ex, 8am every day: '0 15 * * *'" && exit 1

IFS=' ' read -ra toks < <(echo "$CRON_SCHEDULE_DOWN")
[[ ${#toks[@]} -ne 5 ]] && echo "ERROR: Invalid CRON_SCHEDULE_DOWN, cron schedule must be 5 elements, ex, 8am every day: '0 15 * * *'" && exit 1

TMPDIR=$(mktemp -d)
mkdir -p "${TMPDIR}/manifests"
CRONJOB_UP="${TMPDIR}/manifests/cronjob-up.yaml"
CRONJOB_DOWN="${TMPDIR}/manifests/cronjob-down.yaml"
CLOUDBUILD="${TMPDIR}/cloudbuild.yaml"

function makeCronJob() {
  local dest="$1"
  local name_postfix="$2"
  local schedule="$3"
  local min_nodes="$4"
  local max_nodes="$5"

  cat - > $dest <<EOF
kind: CronJob
apiVersion: batch/v1beta1
metadata:
  name: ${NODE_POOL}-node-pool-autoscaler${name_postfix}
  namespace: pod-broker-system
  labels:
    k8s-app: ${NODE_POOL}-node-pool-autoscaler${name_postfix}
spec:
  # UTC time,
  schedule: "${schedule}"
  startingDeadlineSeconds: 3600
  concurrencyPolicy: Replace
  successfulJobsHistoryLimit: 0
  jobTemplate:
    spec:
      activeDeadlineSeconds: 1800
      template:
        spec:
          serviceAccount: pod-broker
          restartPolicy: OnFailure
          nodeSelector:
            cloud.google.com/gke-nodepool: "${NODE_POOL}"
          tolerations:
            - key: "app.broker/tier"
              effect: "NoSchedule"
              operator: "Exists"
            - key: "app.broker/node-init"
              effect: "NoSchedule"
              operator: "Exists"
            - key: "nvidia.com/gpu"
              effect: "NoSchedule"
              operator: "Exists"
            - key: "cloud.google.com/gke-accelerator-init"
              effect: "NoSchedule"
              operator: "Exists"
          containers:
            ###
            # autoscaler container
            ###
            - image: google/cloud-sdk:alpine
              name: autoscaler
              command: ["/bin/bash"]
              args:
                - "-exc"
                - |
                  gcloud container node-pools update ${NODE_POOL} --region=${REGION} --cluster=broker-${REGION} --enable-autoscaling --min-nodes=${min_nodes} --max-nodes=${max_nodes}
              resources:
                requests:
                  cpu: 10m
EOF
}

makeCronJob $CRONJOB_UP "-up" "${CRON_SCHEDULE_UP}" $MIN_NODES $MAX_NODES
makeCronJob $CRONJOB_DOWN "-down" "${CRON_SCHEDULE_DOWN}" 0 $MAX_NODES

cat - > $CLOUDBUILD <<'EOF'
timeout: 3600s
substitutions:
  _INFRA_NAME: broker
  _REGION:
  _NODE_POOL:
tags:
  - autoscaling-cronjob
  - selkies-setup
steps:
  - name: "gcr.io/cloud-builders/gcloud"
    id: "verify-node-pool"
    entrypoint: "bash"
    args:
      - "-exc"
      - |
        RES=$$(gcloud -q container node-pools list --region ${_REGION} --cluster ${_INFRA_NAME}-${_REGION} --filter="name~${_NODE_POOL}" --format='value(name)')
        [[ -z "$$RES" ]] && echo "ERROR: Node pool not found: ${_NODE_POOL}" && exit 1
        exit 0
  - name: "gcr.io/cloud-builders/kubectl"
    id: "deploy-autoscaling-cronjob"
    args: ["apply", "-f", "manifests/"]
    env:
      - "CLOUDSDK_CORE_PROJECT=${PROJECT_ID}"
      - "CLOUDSDK_COMPUTE_REGION=${_REGION}"
      - "CLOUDSDK_CONTAINER_CLUSTER=broker-${_REGION}"
EOF

cd $TMPDIR
gcloud builds submit --substitutions=_REGION=${REGION},_NODE_POOL=${NODE_POOL} 

rm -rf $TMPDIR