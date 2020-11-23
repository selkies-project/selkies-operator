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

export CYAN='\033[1;36m'
export GREEN='\033[1;32m'
export RED='\033[1;31m'
export NC='\033[0m' # No Color
function log_cyan() { echo -e "${CYAN}$@${NC}"; }
function log_green() { echo -e "${GREEN}$@${NC}"; }
function log_red() { echo -e "${RED}$@${NC}"; }

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
