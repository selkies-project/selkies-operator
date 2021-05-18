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
set -o pipefail

port_forward1_pid=$!
function cleanup() {
    echo "stopping port-forward"
    kill -9 $port_forward1_pid
    kill -9 $port_forward2_pid
    echo "done"
}
trap cleanup EXIT

POD=$(kubectl $CTX get pod -n pod-broker-system -l app=pod-broker -o jsonpath='{..metadata.name}')
[[ -z "${POD}" ]] && echo "ERROR: failed to get pod-broker pod from cluster" && exit 1

kubectl port-forward -n pod-broker-system ${POD} --address 0.0.0.0 8080:8080 &
port_forward1_pid=$!
kubectl port-forward -n pod-broker-system ${POD} --address 0.0.0.0 8084:8082 &
port_forward2_pid=$!

if [[ ! -d src/vendor ]]; then
    mkdir -p src/vendor
    curl -sSL "https://cdnjs.cloudflare.com/ajax/libs/vuetify/2.1.12/vuetify.min.css" -o src/vendor/vuetify.min.css
    curl -sSL "https://cdnjs.cloudflare.com/ajax/libs/vue/2.6.9/vue.min.js" -o src/vendor/vue.min.js
    curl -sSL "https://cdnjs.cloudflare.com/ajax/libs/vuetify/2.1.12/vuetify.min.js" -o src/vendor/vuetify.min.js
    curl -sSL "https://cdn.jsdelivr.net/npm/vue-spinner@1.0.3/dist/vue-spinner.min.js" -o src/vendor/vue-spinner.min.js
fi

USER_ACCOUNT=${USER_ACCOUNT:-$(gcloud config get-value account)}

cat - | node <<EOF
var express = require('express');
var proxy = require('http-proxy-middleware');
var app = express();
app.use('/', express.static('src'))
app.use(
    '/broker',
    proxy.createProxyMiddleware(
        {
            target: 'http://localhost:8080',
            changeOrigin: true,
            pathRewrite: {
                '^/broker': '/'
            },
            onProxyReq: (proxyReq, req, res) => {
                proxyReq.setHeader('x-goog-authenticated-user-email', '${USER_ACCOUNT}')
            },
        })
);
app.use(
    '/reservation-broker',
    proxy.createProxyMiddleware(
        {
            target: 'http://localhost:8084',
            changeOrigin: true,
            pathRewrite: {
                '^/reservation-broker': '/'
            },
        })
);
var devHost = process.env['WEB_PREVIEW_PORT_3000'] ? process.env['WEB_PREVIEW_PORT_3000'] : "http://localhost:3000";
console.log("dev server listening at: " + devHost);
app.listen(3000);
EOF