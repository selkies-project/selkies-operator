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

FROM golang:1.15-alpine as build
RUN apk add --no-cache -u git
ENV GO111MODULE=on
WORKDIR /go/src/selkies.io/controller
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build cmd/app_finder/app_finder.go
RUN go build cmd/app_publisher/app_publisher.go
RUN go build cmd/image_finder/image_finder.go
RUN go build cmd/image_puller/image_puller.go
RUN go build cmd/pod_broker/pod_broker.go
RUN go build cmd/reservation_broker/reservation_broker.go

FROM gcr.io/k8s-skaffold/skaffold:v1.10.1 as skaffold
FROM gcr.io/google.com/cloudsdktool/cloud-sdk:alpine

# Install kustomize
COPY --from=skaffold /usr/local/bin/kustomize /usr/local/bin/kustomize

# Install kubectl
COPY --from=skaffold /usr/local/bin/kubectl /usr/local/bin/kubectl

# Copy build from previous layer
COPY --from=build /go/src/selkies.io/controller/app_finder /usr/local/bin/app-finder
COPY --from=build /go/src/selkies.io/controller/app_publisher /usr/local/bin/app-publisher
COPY --from=build /go/src/selkies.io/controller/image_finder /usr/local/bin/image-finder
COPY --from=build /go/src/selkies.io/controller/image_puller /usr/local/bin/image-puller
COPY --from=build /go/src/selkies.io/controller/pod_broker /usr/local/bin/pod-broker
COPY --from=build /go/src/selkies.io/controller/reservation_broker /usr/local/bin/reservation-broker

# Copy build assets
WORKDIR /opt/broker/buildsrc/
COPY config .

WORKDIR /var/run/build

ENTRYPOINT ["/usr/local/bin/pod-broker"]
