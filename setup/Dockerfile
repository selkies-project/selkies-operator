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

FROM gcr.io/cloud-builders/gke-deploy as gke-deploy
FROM gcr.io/cloud-builders/kubectl as kubectl

FROM gcr.io/google.com/cloudsdktool/cloud-sdk:alpine
RUN apk add -u \
    jq

ARG TERRAFORM_VERSION=0.12.20
ARG KUSTOMIZE_VERSION=3.5.3
ARG ISTIO_OPERATOR_VERSION=1.4.7
ARG CNRM_VERSION=1.7.1

# Install terraform
RUN wget --timeout 2 --tries 5 https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    unzip terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    chmod +x terraform && \
    mv terraform /usr/local/bin && \
    rm -rf terraform_${TERRAFORM_VERSION}_linux_amd64.zip

# Install kustomize
RUN wget --timeout 2 --tries 5 -O- https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz | \
    tar zxvf - kustomize && \
    chmod +x kustomize && \
    mv kustomize /usr/local/bin

# Install gke-deploy
COPY --from=gke-deploy /gke-deploy /usr/local/bin/

# Install kubectl
COPY --from=kubectl /builder/google-cloud-sdk/bin/kubectl /usr/local/bin/kubectl

# Download cnrm installer
RUN mkdir -p /opt/cnrm && cd /opt/cnrm && \
    wget --timeout 2 --tries 5 -qO- https://storage.googleapis.com/cnrm/${CNRM_VERSION}/release-bundle.tar.gz | tar -zxvf - ./install-bundle-workload-identity/

# Download the istio operator release
RUN mkdir -p /opt/istio-operator && cd /opt/istio-operator && \
    wget --timeout 2 --tries 5 -qO- https://github.com/istio/operator/archive/${ISTIO_OPERATOR_VERSION}.tar.gz | tar --strip-components=1 -zxf - 
