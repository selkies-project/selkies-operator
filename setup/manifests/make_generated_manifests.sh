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
# Fetch auth header values from Secret Manager
###
AUTH_HEADER="x-goog-authenticated-user-email"
AUTH_HEADER_SECRET_VERSION=$(gcloud -q secrets versions list broker-auth-header --sort-by=created --limit=1 --format='value(name)' 2>/dev/null || true)
if [[ -n "${AUTH_HEADER_SECRET_VERSION}" ]]; then
  AUTH_HEADER=$(gcloud secrets versions access ${AUTH_HEADER_SECRET_VERSION} --secret broker-auth-header)
fi

USERNAME_HEADER="x-broker-user"
USERNAME_HEADER_SECRET_VERSION=$(gcloud -q secrets versions list broker-username-header --sort-by=created --limit=1 --format='value(name)' 2>/dev/null || true)
if [[ -n "${USERNAME_HEADER_SECRET_VERSION}" ]]; then
  USERNAME_HEADER=$(gcloud secrets versions access ${USERNAME_HEADER_SECRET_VERSION} --secret broker-username-header)
fi

###
# Broker configmap items
###
CONFIG_DATA=$(cat <<-EOF
  POD_BROKER_PARAM_ProjectID: "${PROJECT_ID}"
  POD_BROKER_PARAM_Theme: "dark"
  POD_BROKER_PARAM_Title: "App Launcher"
  POD_BROKER_PARAM_Domain: "${ENDPOINT}"
  POD_BROKER_PARAM_AuthHeader: "${AUTH_HEADER}"
  POD_BROKER_PARAM_UsernameHeader: "${USERNAME_HEADER}"
  POD_BROKER_PARAM_AuthorizedUserRepoPattern: "gcr.io/.*"
EOF
)

DEST=${DEST_DIR}/patch-pod-broker-config.yaml
cat > "${DEST}" << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: pod-broker-config
data:
${CONFIG_DATA}
EOF

echo "INFO: Created pod broker config patch: ${DEST}"

# Generate md5 hash of configmap data to enable rolling updates of pod-broker when config changes.
BROKER_CONFIG_HASH=$(echo "$CONFIG_DATA" | md5sum | cut -d' ' -f1)

DEST=${DEST_DIR}/patch-pod-broker-config-hash.yaml
cat > "${DEST}" << EOF
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: pod-broker
spec:
  template:
    metadata:
      annotations:
        app.broker/config-hash: "${BROKER_CONFIG_HASH}"
EOF

echo "INFO: Created pod broker configmap hash patch: ${DEST}"

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
# Patch to add cluser service account email to pod-broker-node-init service account annotation.
# This enables the GKE Workload Identity feature for the pod-broker-node-init pod.
###
DEST="${DEST_DIR}/patch-pod-broker-node-init-service-account.yaml"
cat > "${DEST}" << EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pod-broker-node-init
  namespace: kube-system
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
# Extract the latest image tag digests
# This enables rolling updates.
###
function fetchLatestDigest() {
  local image=$1
  local digest=$(gcloud -q container images list-tags $image --limit 1 --filter=tags~latest --format="json" | jq -r ".[].digest")
  [[ $? -ne 0 || -z "$digest" ]] && echo "ERROR: failed to find digest for ${image}:latest" >&2 && return 1
  echo "${image}@${digest}"
}

CONTROLLER_IMAGE=${CONTROLLER_IMAGE:-$(fetchLatestDigest gcr.io/${PROJECT_ID}/kube-pod-broker-controller)}
WEB_IMAGE=${WEB_IMAGE:-$(fetchLatestDigest gcr.io/${PROJECT_ID}/kube-pod-broker-web)}
COTURN_IMAGE=${COTURN_IMAGE:-$(fetchLatestDigest gcr.io/${PROJECT_ID}/kube-pod-broker-coturn)}
COTURN_WEB_IMAGE=${COTURN_WEB_IMAGE:-$(fetchLatestDigest gcr.io/${PROJECT_ID}/kube-pod-broker-coturn-web)}

###
# Generate kustomization file with project scoped images.
###
(
  cd ${DEST_DIR}
  rm -f kustomization.yaml
  kustomize create
  # TODO: need to remove this commonLabel as it causes issues with headless services (turn-discovery).
  # Simply removing it breaks the ability to apply this change as an update operation because the labeled fields are immutable.
  kustomize edit add label "app.kubernetes.io/name":"${INFRA_NAME}"
  kustomize edit add base "../base/custom-metrics/"
  kustomize edit add base "../base/ingress/"
  kustomize edit add base "../base/node/"
  kustomize edit add base "../base/pod-broker/"
  kustomize edit add base "../base/turn/"
  kustomize edit add patch "patch-pod-broker-config.yaml"
  kustomize edit add patch "patch-pod-broker-config-hash.yaml"
  kustomize edit add patch "patch-pod-broker-service-account.yaml"
  kustomize edit add patch "patch-pod-broker-node-init-service-account.yaml"
  kustomize edit add patch "patch-pod-broker-gateway.yaml"
  kustomize edit add patch "patch-pod-broker-virtual-service.yaml"
  kustomize edit set image \
    gcr.io/cloud-solutions-images/kube-pod-broker-controller:latest=${CONTROLLER_IMAGE} \
    gcr.io/cloud-solutions-images/kube-pod-broker-web:latest=${WEB_IMAGE} \
    gcr.io/cloud-solutions-images/kube-pod-broker-coturn:latest=${COTURN_IMAGE} \
    gcr.io/cloud-solutions-images/kube-pod-broker-coturn-web:latest=${COTURN_WEB_IMAGE}
)
