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

FROM gcr.io/cloud-builders/gke-deploy as gke-deploy

FROM gcr.io/google.com/cloudsdktool/cloud-sdk:alpine
RUN apk add -u \
    jq

ENV TERRAFORM_VERSION=0.12.20
ENV KUBECTL_VERSION=1.16.1
ENV KUSTOMIZE_VERSION=3.5.3
ENV ISTIO_OPERATOR_VERSION=1.4.3
ENV CNRM_VERSION=1.0.0

# Install terraform
RUN wget https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    unzip terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    chmod +x terraform && \
    mv terraform /usr/local/bin && \
    rm -rf terraform_${TERRAFORM_VERSION}_linux_amd64.zip

# Install kubectl
RUN wget https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/

# Install kustomize
RUN wget -O- https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz | \
    tar zxvf - kustomize && \
    chmod +x kustomize && \
    mv kustomize /usr/local/bin

# Install gke-deploy
COPY --from=gke-deploy /gke-deploy /usr/local/bin/

# Download cnrm installer
RUN mkdir -p /opt/cnrm && cd /opt/cnrm && \
    wget -qO- https://storage.googleapis.com/cnrm/${CNRM_VERSION}/release-bundle.tar.gz | tar -zxvf - ./install-bundle-workload-identity/

# Download the istio operator release
RUN mkdir -p /opt/istio-operator && cd /opt/istio-operator && \
    wget -qO- https://github.com/istio/operator/archive/${ISTIO_OPERATOR_VERSION}.tar.gz | tar --strip-components=1 -zxf - 
