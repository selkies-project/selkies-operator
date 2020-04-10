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

FROM golang:1.13-alpine as build
RUN apk add -u git
ENV GO111MODULE=on
WORKDIR /go/src/gcp.solutions/kube-app-launcher
COPY . ./
RUN go build cmd/app_finder/app_finder.go
RUN go build cmd/image_finder/image_finder.go
RUN go build cmd/image_puller/image_puller.go
RUN go build cmd/pod_broker/pod_broker.go

FROM gcr.io/google.com/cloudsdktool/cloud-sdk:alpine

# Install kubectl with integrated kustomize 
RUN wget -qO /usr/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.18.1/bin/linux/amd64/kubectl && \
    chmod +x /usr/bin/kubectl

# Copy build from previous layer
COPY --from=build /go/src/gcp.solutions/kube-app-launcher/app_finder /usr/local/bin/app-finder
COPY --from=build /go/src/gcp.solutions/kube-app-launcher/image_finder /usr/local/bin/image-finder
COPY --from=build /go/src/gcp.solutions/kube-app-launcher/image_puller /usr/local/bin/image-puller
COPY --from=build /go/src/gcp.solutions/kube-app-launcher/pod_broker /usr/local/bin/pod-broker

# Copy build assets
WORKDIR /opt/broker/buildsrc/
COPY config .

WORKDIR /var/run/build

ENTRYPOINT ["/usr/local/bin/pod-broker"]
