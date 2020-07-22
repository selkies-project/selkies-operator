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
WORKDIR /go/src/coturn-web-aggregator
COPY *.go ./
RUN go build

FROM alpine:3.9

# Copy build from previous layer
COPY --from=build /go/src/coturn-web-aggregator/coturn-web-aggregator /usr/local/bin/coturn-web-aggregator

ENTRYPOINT ["/usr/local/bin/coturn-web-aggregator"]