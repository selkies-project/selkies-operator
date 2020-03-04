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

# Script used to create generated project/user specific assets that are not tracked by git.

set -e

IMAGE_TAG=$1
[[ -z "${IMAGE_TAG}" ]] && echo "USAGE: $0 <image tag>" && exit 1

SCRIPT_DIR=$(dirname $(readlink -f $0 2>/dev/null) 2>/dev/null || echo "${PWD}/$(dirname $0)")

DEST_DIR="${SCRIPT_DIR}/generated"
mkdir -p "${DEST_DIR}"

[[ ! -f terraform.tfstate ]] && echo "ERROR: terraform.tfstate file not found" && exit 1

export NAMESPACE="default"
export PROJECT_ID=${PROJECT_ID:-$(terraform output project_id)}
export INFRA_NAME=$(terraform output name)
export STATIC_IP_NAME=$(terraform output static-ip-name)
export NODE_SERVICE_ACCOUNT=$(terraform output node-service-account)
export ENDPOINT=$(terraform output cloud-ep-endpoint)
export CLOUD_DNS=$(terraform output cloud-dns)
if [[ -n "${CLOUD_DNS}" ]]; then
  ENDPOINT=${CLOUD_DNS%?}
fi

###
# Broker configmap items
###
DEST=${DEST_DIR}/patch-pod-broker-config.yaml
cat > "${DEST}" << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: pod-broker-config
data:
  POD_BROKER_PARAM_Theme: "dark"
  POD_BROKER_PARAM_Title: "App Launcher"
  POD_BROKER_PARAM_Domain: "${ENDPOINT}"
  POD_BROKER_PARAM_AuthHeader: "x-goog-authenticated-user-email"
  POD_BROKER_PARAM_AuthorizedUserRepoPattern: "gcr.io/.*"
EOF

echo "INFO: Created pod broker config patch: ${DEST}"

###
# Patch to add cluser service account email to pod-broker service account annotation.
# This enables the GKE Workload Identity feature for the pod-broker.
###
DEST="${DEST_DIR}/patch-pod-broker-service-account.yaml"
cat > "${DEST}" << EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pod-broker
  annotations:
    iam.gke.io/gcp-service-account: "${NODE_SERVICE_ACCOUNT}"
EOF

echo "INFO: Created pod broker service account patch: ${DEST}"

###
# Patch to add host to istio Gateway for pod broker.
###
DEST="${DEST_DIR}/patch-pod-broker-gateway.yaml"

cat > "${DEST}" << EOF
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: pod-broker-gateway
spec:
  selector:
    istio: ingressgateway
  servers:
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - "${ENDPOINT}"
        - "*.${ENDPOINT}"
EOF

echo "INFO: Created pod broker gateway patch: ${DEST}"

###
# Patch to add exact endpoint to pod broker vritual service
###
DEST="${DEST_DIR}/patch-pod-broker-virtual-service.yaml"

cat > "${DEST}" << EOF
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: pod-broker
spec:
  hosts:
    - "${ENDPOINT}"
EOF

echo "INFO: Created pod broker virtualservice patch: ${DEST}"

###
# Generate kustomization file with project scoped images.
###
(
  cd ${DEST_DIR}
  rm -f kustomization.yaml
  kustomize create
  kustomize edit add label "app.kubernetes.io/name":"${INFRA_NAME}"
  kustomize edit add base "../base/ingress/"
  kustomize edit add base "../base/node/"
  kustomize edit add base "../base/pod-broker/"
  kustomize edit add base "../base/turn/"
  kustomize edit add patch "patch-pod-broker-config.yaml"
  kustomize edit add patch "patch-pod-broker-service-account.yaml"
  kustomize edit add patch "patch-pod-broker-gateway.yaml"
  kustomize edit add patch "patch-pod-broker-virtual-service.yaml"
  kustomize edit set image \
    gcr.io/cloud-solutions-images/kube-pod-broker-controller:latest=gcr.io/${PROJECT_ID}/kube-pod-broker-controller:${IMAGE_TAG} \
    gcr.io/cloud-solutions-images/kube-pod-broker-web:latest=gcr.io/${PROJECT_ID}/kube-pod-broker-web:${IMAGE_TAG} \
    gcr.io/cloud-solutions-images/kube-pod-broker-coturn:latest=gcr.io/${PROJECT_ID}/kube-pod-broker-coturn:${IMAGE_TAG} \
    gcr.io/cloud-solutions-images/kube-pod-broker-coturn-web:latest=gcr.io/${PROJECT_ID}/kube-pod-broker-coturn-web:${IMAGE_TAG}
)
