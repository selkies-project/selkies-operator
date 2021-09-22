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

set -e

SCRIPT_DIR=$(dirname $(readlink -f $0 2>/dev/null) 2>/dev/null || echo "${PWD}/$(dirname $0)")

cd "${SCRIPT_DIR}"

PROJECT_ID=${PROJECT_ID?}
INFRA_NAME=${INFRA_NAME?}
CLUSTER_LOCATION=${REGION?}
CLUSTER_NAME=${INFRA_NAME}-${CLUSTER_LOCATION}

export CYAN='\033[1;36m'
export GREEN='\033[1;32m'
export RED='\033[1;31m'
export NC='\033[0m' # No Color
function log_cyan() { echo -e "${CYAN}$@${NC}"; }
function log_green() { echo -e "${GREEN}$@${NC}"; }
function log_red() { echo -e "${RED}$@${NC}"; }

export ISTIOCTL=/opt/istio-${LATEST_ISTIO}/bin/istioctl
export ISTIOCTL_COMPAT=/opt/istio-${ISTIO_COMPAT}/bin/istioctl

# Extract endpoint and backend service name from terraform output.
ENDPOINT="broker.endpoints.${PROJECT_ID}.cloud.goog"
BACKEND_SERVICE="istio-ingressgateway"
TFSTATE="$(gsutil cat gs://${PROJECT_ID}-broker-tf-state/${INFRA_NAME}/lb-${CLUSTER_LOCATION}.tfstate 2>/dev/null || true)"
if [[ -n "${TFSTATE}" ]]; then
    # Use endpoint and backend service from regional LB.
    ENDPOINT=$(jq -r '.outputs["cloud-ep-endpoint"].value' <<< $TFSTATE)
    [[ -z "${ENDPOINT}" || "${ENDPOINT,,}" == "null" ]] && log_red "ERROR: Failed to get regional LB endpoint from tfstate" && exit 1
    BACKEND_SERVICE=$(jq -r '.outputs["backend-service"].value' <<< $TFSTATE)
    [[ -z "${BACKEND_SERVICE}" ]] && log_red "ERROR: Failed to get regional LB backend service name from tfstate" && exit 1
fi

# Override ENDPOINT from Secret Manager
CUSTOM_DOMAIN=""
CUSTOM_DOMAIN_SECRET_VERSION=$(gcloud -q secrets versions list broker-custom-domain --sort-by=created --limit=1 --format='value(name)' 2>/dev/null || true)
if [[ -n "${CUSTOM_DOMAIN_SECRET_VERSION}" ]]; then
    CUSTOM_DOMAIN=$(gcloud secrets versions access ${CUSTOM_DOMAIN_SECRET_VERSION} --secret broker-custom-domain | xargs)
fi
if [[ -n "${CUSTOM_DOMAIN}" ]]; then
    ENDPOINT="${CUSTOM_DOMAIN}"
    log_green "INFO: Using custom domain for endpoint: '${ENDPOINT}'"
fi
export ENDPOINT

# Get cluster credentials
log_cyan "Obtaining cluster credentials..."
gcloud container clusters get-credentials ${CLUSTER_NAME} --region=${CLUSTER_LOCATION}

# Install CRDs
log_cyan "Installing CRDs"
gke-deploy apply --project ${PROJECT_ID} --cluster ${CLUSTER_NAME} --location ${CLUSTER_LOCATION} --filename /opt/istio-operator/deploy/crds/istio_v1alpha2_istiocontrolplane_crd.yaml
gke-deploy apply --project ${PROJECT_ID} --cluster ${CLUSTER_NAME} --location ${CLUSTER_LOCATION} --filename base/pod-broker/crd.yaml

# Install CNRM controller
kubectl apply -f /opt/cnrm/install-bundle-workload-identity/crds.yaml
mkdir -p base/cnrm-system/install-bundle
cp /opt/cnrm/install-bundle-workload-identity/0-cnrm-system.yaml base/cnrm-system/install-bundle/
sed -i 's/${PROJECT_ID?}/'${PROJECT_ID}'/g' \
    base/cnrm-system/install-bundle/0-cnrm-system.yaml \
    base/cnrm-system/patch-cnrm-system-namespace.yaml
kubectl apply -k base/cnrm-system/

# Wait for CNRM controller
log_cyan "Waiting for pod 'cnrm-controller-manager-0'"
until [[ -n $(kubectl get pod cnrm-controller-manager-0 -n cnrm-system -oname 2>/dev/null) ]]; do sleep 2; done
kubectl wait pod cnrm-controller-manager-0 -n cnrm-system --for=condition=Ready --timeout=60s
log_cyan "Pod 'cnrm-controller-manager-0' is ready"

log_cyan "Waiting for Deployment 'cnrm-webhook-manager'"
kubectl wait deploy cnrm-webhook-manager -n cnrm-system --for=condition=Available --timeout=600s
kubectl wait pod -n cnrm-system -l cnrm.cloud.google.com/component=cnrm-webhook-manager --for=condition=Ready --timeout=600s
log_cyan "Deployment 'cnrm-webhook-manager' is ready"

# Install AutoNEG controller
log_cyan "Installing AutoNEG controller..."
kubectl kustomize base/autoneg-system | sed 's/${PROJECT_ID}/'${PROJECT_ID}'/g' | \
    kubectl apply -f -

# Update istio ingressgateway service annotation with backend service name for autoneg.
log_cyan "Updating ingress gateway autoneg annotation to match backend service: ${BACKEND_SERVICE}"
sed -i \
    -e "s|anthos.cft.dev/autoneg:.*|anthos.cft.dev/autoneg: '{\"name\":\"${BACKEND_SERVICE}\", \"max_rate_per_endpoint\":100}'|g" \
        base/istio/istiocontrolplane.yaml base/istio/istiooperator-*.yaml

# Check installed istio version, default is latest.
log_cyan "Checking existing istio installation"
ISTIO_VERSION=$(${ISTIOCTL_COMPAT} version -o json | grep -v "no running Istio" | jq -r '.meshVersion[0].Info.version // "'${LATEST_ISTIO}'"')
ISTIO_LATEST_INSTALLER="./install_istio_${LATEST_ISTIO_MAJOR}.sh"
case "$ISTIO_VERSION" in
    1.4*) log_cyan "Installing istio 1.4" && ./install_istio_1.4.sh ;;
    1.7*) log_cyan "Installing istio 1.7" && ./install_istio_1.7.sh ;;
    * ) log_red "Unsupported istio version found: ${ISTIO_VERSION}, attempting latest installer." && ${ISTIO_LATEST_INSTALLER} ;;
esac

# force repair of autoneg annotations.
log_cyan "Repairing neg-status and autoneg-status annotations on istio-ingressgateway service to force update"
kubectl annotate service istio-ingressgateway -n istio-system anthos.cft.dev/autoneg-status-

# Create generated manifests
log_cyan "Generating manifest kustomizations..."
./make_generated_manifests.sh
cat generated/kustomization.yaml

# If the image cache loader daemonset is present, patch the image puller to wait for it.
if [[ -n "$(kubectl get ds -n kube-system -l app=pod-broker-image-loader -o name)" ]]; then
    log_cyan "Adding patch to image puller to wait for image cache."
    (cd base/pod-broker/image-puller && kustomize edit add patch patch-wait-for-image-cache.yaml)
fi

# Delete old pod-broker StatefulSet, migrate to Deployment
log_cyan "Removing any old pod-broker StatefulSet to migrate to Deployment..."
kubectl delete statefulset -n pod-broker-system pod-broker 2>/dev/null || true

# Delete non-csi StorageClasses, migrate to CSI provisioner.
# The provisioner field of the StorageClass object is immutable, so the old one has to be deleted first.
for sc in pd-ssd pd-standard; do
    PROVISIONER=$(kubectl get storageclass ${sc} -o jsonpath='{.provisioner}' || true)
    if [[ "${PROVISIONER}" == "kubernetes.io/gce-pd" ]]; then
        log_cyan "Removing non-csi storageclass ${sc} to migrate to CSI provisioner"
        kubectl delete storageclass ${sc}
    fi
done

# Apply ip-masq-agent config to fix rfc-1918 pod cidr range on some clusters.
log_cyan "Applying ip-masq-agent patch..."
./fix_pod_cidr_masq.sh || true

# Apply the manifests
log_cyan "Applying manifests..."
kustomize build generated/ | sed -e 's/${PROJECT_ID}/'${PROJECT_ID}'/g' | \
    kubectl apply -f -

log_green "Done"