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

PROJECT_ID=${PROJECT_ID?}
INFRA_NAME=${INFRA_NAME:?}
IMAGE_TAG=${IMAGE_TAG?}
CLUSTER_LOCATION=${REGION?}
CLUSTER_NAME=${INFRA_NAME}-${CLUSTER_LOCATION}

export CYAN='\033[1;36m'
export GREEN='\033[1;32m'
export NC='\033[0m' # No Color
function log_cyan() { echo -e "${CYAN}$@${NC}"; }
function log_green() { echo -e "${GREEN}$@${NC}"; }

# Get cluster credentials
log_cyan "Obtaining cluster credentials..."
gcloud container clusters get-credentials ${CLUSTER_NAME} --region=${CLUSTER_LOCATION}

# Install CRDs
log_cyan "Installing CRDs"
gke-deploy apply --project ${PROJECT_ID} --cluster ${CLUSTER_NAME} --location ${CLUSTER_LOCATION} --filename /opt/istio-operator/deploy/crds/istio_v1alpha2_istiocontrolplane_crd.yaml
gke-deploy apply --project ${PROJECT_ID} --cluster ${CLUSTER_NAME} --location ${CLUSTER_LOCATION} --filename base/pod-broker/crd.yaml

# Install AutoNEG controller
log_cyan "Installing AutoNEG controller..."
kubectl kustomize base/autoneg-system | sed 's/${PROJECT_ID}/'${PROJECT_ID}'/g' | \
    kubectl apply -f -

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
./make_generated_manifests.sh
cat generated/kustomization.yaml

# Apply the manifests
log_cyan "Applying manifests..."
kustomize build generated/ | kubectl apply -f -

log_green "Done"