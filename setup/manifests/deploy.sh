#!/bin/bash

# Copyright 2019 Google Inc. All rights reserved.
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

PROJECT_ID=$1
INFRA_NAME=$2
INGRESS_FLAVOR=$3
IMAGE_TAG=$4

[[ $# -ne 4 ]] && echo "USAGE: $0 <PROJECT_ID> <INFRA_NAME> <INGRESS_FLAVOR> <IMAGE_TAG>" && exit 1

export CYAN='\033[1;36m'
export GREEN='\033[1;32m'
export NC='\033[0m' # No Color
function log_cyan() { echo -e "${CYAN}$@${NC}"; }
function log_green() { echo -e "${GREEN}$@${NC}"; }

# Copy terraform state file to access output variables from infrastructure.
log_cyan "Fetching terraform state file..."
TFSTATE="gs://${PROJECT_ID}-${INFRA_NAME}-tf-state/${INFRA_NAME}/default.tfstate"
gsutil cp ${TFSTATE} ./terraform.tfstate
terraform output

# Cluster variables
broker_west_cluster_name=$(terraform output broker-west-cluster-name)
broker_west_cluster_location=$(terraform output broker-west-cluster-location)

# Get cluster credentials for us-west1
log_cyan "Obtaining cluster credentials..."
gcloud container clusters get-credentials ${broker_west_cluster_name} --region=${broker_west_cluster_location}

# Install CRDs
log_cyan "Installing CRDs"
gke-deploy apply --project ${PROJECT_ID} --cluster ${broker_west_cluster_name} --location ${broker_west_cluster_location} --filename /opt/istio-operator/deploy/crds/istio_v1alpha2_istiocontrolplane_crd.yaml
gke-deploy apply --project ${PROJECT_ID} --cluster ${broker_west_cluster_name} --location ${broker_west_cluster_location} --filename base/pod-broker/crd.yaml

# apply CNRM CRDs with kubectl because gke-deploy is very slow.
kubectl apply -f /opt/cnrm/install-bundle-workload-identity/crds.yaml

# Install CNRM controller
mkdir -p base/cnrm-system/install-bundle
cp /opt/cnrm/install-bundle-workload-identity/0-cnrm-system.yaml base/cnrm-system/install-bundle/
sed -i 's/${PROJECT_ID?}/'${PROJECT_ID}'/g' \
    base/cnrm-system/install-bundle/0-cnrm-system.yaml \
    base/cnrm-system/patch-cnrm-system-namespace.yaml
kubectl apply -k base/cnrm-system/

# Install istio operator
log_cyan "Installing Istio operator..."
kubectl apply -k /opt/istio-operator/deploy/

# Create istio control plane
log_cyan "Creating Istio control plane..."
kubectl apply -f base/istio/istiocontrolplane.yaml
 
# Wait for operator to create istio control plane objects
# Objects created async by the operator may not exist yet.
# Manual wait for object creation until this is merged: https://github.com/kubernetes/kubernetes/pull/83335
log_cyan "Waiting for namespace 'istio-system'"
until [[ -n $(kubectl get namespace istio-system -oname 2>/dev/null) ]]; do sleep 2; done
log_cyan "Namespace 'istio-system' is ready"

log_cyan "Waiting for istio controlplane crds"
until [[ -n $(kubectl get crd gateways.networking.istio.io -oname 2>/dev/null) ]]; do sleep 2; done
until [[ -n $(kubectl get crd virtualservices.networking.istio.io -oname 2>/dev/null) ]]; do sleep 2; done
log_cyan "Istio control plane crds are ready"

# Create generated manifests
log_cyan "Generating manifest kustomizations..."
export CLIENT_ID=${OAUTH_CLIENT_ID}
export CLIENT_SECRET=${OAUTH_CLIENT_SECRET}
./make_generated_manifests.sh ${INGRESS_FLAVOR} ${IMAGE_TAG}
cat generated/kustomization.yaml

# Create the ingress resources
log_cyan "Applying ingress manifests..."
kubectl apply -k base/ingress/iap-ingress/

# Apply the manifests
log_cyan "Applying manifests..."
kustomize build generated/ | kubectl apply -f -

# Display the broker URL
ENDPOINT=$(terraform output cloud-ep-endpoint)
log_green "Done. Broker URL https://${ENDPOINT}"