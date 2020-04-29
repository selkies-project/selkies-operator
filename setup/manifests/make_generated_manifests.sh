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

# Script used to create generated project/user specific assets that are not tracked by git.

set -e

PROJECT_ID=${PROJECT_ID?}
INFRA_NAME=${INFRA_NAME?}
IMAGE_TAG=${IMAGE_TAG?}
NODE_SERVICE_ACCOUNT=${NODE_SERVICE_ACCOUNT?}
ENDPOINT=${ENDPOINT?}

SCRIPT_DIR=$(dirname $(readlink -f $0 2>/dev/null) 2>/dev/null || echo "${PWD}/$(dirname $0)")

DEST_DIR="${SCRIPT_DIR}/generated"
mkdir -p "${DEST_DIR}"

###
# Fetch broker cookie secret from Secret Manager
###
COOKIE_SECRET_VERSION=${COOKIE_SECRET_VERSION:-$(gcloud secrets versions list broker-cookie-secret --sort-by=created --limit=1 --format='value(name)')}
COOKIE_SECRET=$(gcloud secrets versions access ${COOKIE_SECRET_VERSION} --secret broker-cookie-secret)
[[ -z "${COOKIE_SECRET}" ]] && echo "Failed to get broker-cookie-secret from Secret Manager" && exit 1

###
# Fetch OAuth client ID from Secret Manager
###
CLIENT_ID_SECRET_VERSION=${OAUTH_CLIENT_ID_SECRET_VERSION:-$(gcloud secrets versions list broker-oauth2-client-id --sort-by=created --limit=1 --format='value(name)')}
CLIENT_ID=$(gcloud secrets versions access ${CLIENT_ID_SECRET_VERSION} --secret broker-oauth2-client-id)
[[ -z "${CLIENT_ID}" ]] && echo "Failed to get broker-oauth2-client-id from Secret Manager" && exit 1

# Add secrets to pod-broker kustomization
(cd "${SCRIPT_DIR}/base/pod-broker" && kustomize edit add secret pod-broker --from-literal=COOKIE_SECRET=${COOKIE_SECRET})
(cd "${SCRIPT_DIR}/base/pod-broker" && kustomize edit add secret oauth-client-id --from-literal=CLIENT_ID=${CLIENT_ID})

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
