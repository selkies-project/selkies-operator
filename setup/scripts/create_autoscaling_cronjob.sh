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

NODE_POOL=$1
DURATION_HRS=$2
CRON_SCHEDULE=$3
REGION=$4

[[ $# -ne 4 ]] && echo "USAGE: $0 <node pool name> <duration in hours> <chron schedule> <region>" && exit 1

IFS=' ' read -ra toks < <(echo "$CRON_SCHEDULE")
[[ ${#toks[@]} -ne 5 ]] && echo "ERROR: Cron schedule must be 5 elements, ex, 8am every day: '0 15 * * *'" && exit 1

((AUTOSCALE_DURATION=DURATION_HRS * 3600))

[[ $AUTOSCALE_DURATION -lt 1 ]] && echo "ERROR: Invalid autoscaler duration" && exit 1

TMPDIR=$(mktemp -d)
CRONJOB="${TMPDIR}/cronjob.yaml"
CLOUDBUILD="${TMPDIR}/cloudbuild.yaml"

cat - > $CRONJOB <<EOF
kind: CronJob
apiVersion: batch/v1beta1
metadata:
  name: ${NODE_POOL}-node-pool-autoscaler
  namespace: kube-system
  labels:
    k8s-app: ${NODE_POOL}-node-pool-autoscaler
spec:
  # UTC time,
  schedule: "${CRON_SCHEDULE}"
  startingDeadlineSeconds: 3600
  concurrencyPolicy: Replace
  jobTemplate:
    spec:
      # How long in seconds to keep autoscaler up.
      activeDeadlineSeconds: ${AUTOSCALE_DURATION}
      template:
        spec:
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
            # pause container
            ###
            - image: gcr.io/google-containers/pause:2.0
              name: pause
              resources:
                requests:
                  cpu: 10m
EOF

cat - > $CLOUDBUILD <<'EOF'
timeout: 3600s
substitutions:
  _INFRA_NAME: broker
  _REGION:
  _NODE_POOL:

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
    id: "deploy-manifests"
    args: ["apply", "-f", "cronjob.yaml"]
    env:
      - "CLOUDSDK_CORE_PROJECT=${PROJECT_ID}"
      - "CLOUDSDK_COMPUTE_REGION=${_REGION}"
      - "CLOUDSDK_CONTAINER_CLUSTER=broker-${_REGION}"
EOF

cd $TMPDIR
gcloud builds submit --substitutions=_REGION=${REGION},_NODE_POOL=${NODE_POOL} 

rm -rf $TMPDIR