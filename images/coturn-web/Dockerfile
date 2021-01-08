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

FROM golang:1.12-alpine as build
WORKDIR /go/src/coturn-web
COPY *.go ./
RUN go build

FROM gcr.io/k8s-skaffold/skaffold:v1.10.1 as skaffold
FROM alpine:3.9

# Install tools
RUN apk add -u bash jq bind-tools curl

# Copy build from previous layer
COPY --from=build /go/src/coturn-web/coturn-web /usr/local/bin/coturn-web

# Install kubectl
COPY --from=skaffold /usr/local/bin/kubectl /usr/local/bin/kubectl

COPY detect_external_ip.sh /usr/bin/detect_external_ip
RUN chmod +x /usr/bin/detect_external_ip

COPY node_watcher.sh /node_watcher.sh
RUN chmod +x /node_watcher.sh

COPY entrypoint.sh /
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]