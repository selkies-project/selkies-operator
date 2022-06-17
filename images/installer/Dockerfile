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
    jq \
    coreutils

ARG TERRAFORM_VERSION=0.12.30
ARG KUSTOMIZE_VERSION=3.5.3
ARG ISTIO_OPERATOR_VERSION=1.4.7
ARG CNRM_VERSION=1.88.0

# istoctl capable of detecting version 1.4+
ENV ISTIO_COMPAT=1.7.3

# istioctl and istio bundle to be installed if 1.4 not found.
ENV LATEST_ISTIO=1.7.3
ENV LATEST_ISTIO_MAJOR=1.7

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

# Download compat istio release
RUN mkdir -p /opt/istio-${ISTIO_COMPAT}/bin && cd /opt/istio-${ISTIO_COMPAT}/bin && \
    curl -L https://github.com/istio/istio/releases/download/${ISTIO_COMPAT}/istioctl-${ISTIO_COMPAT}-linux-amd64.tar.gz | tar zxvf - istioctl > istioctl && chmod +x istioctl

# Download latest istio releases
RUN cd /opt && curl -L https://istio.io/downloadIstio | ISTIO_VERSION=${LATEST_ISTIO} TARGET_ARCH=x86_64 sh -