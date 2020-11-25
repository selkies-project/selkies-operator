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
kubectl port-forward -n pod-broker-system pod-broker-0 --address 0.0.0.0 8080:8080 &
port_forward1_pid=$!
kubectl port-forward -n pod-broker-system pod-broker-0 --address 0.0.0.0 8082:8082 &
port_forward2_pid=$!

cat - | node <<'EOF'
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
        })
);
app.use(
    '/reservation-broker',
    proxy.createProxyMiddleware(
        {
            target: 'http://localhost:8082',
            changeOrigin: true,
            pathRewrite: {
                '^/reservation-broker': '/'
            },
        })
);
var devHost = process.env['CODE_SERVER_WEB_PREVIEW_3000'] ? "https://" + process.env['CODE_SERVER_WEB_PREVIEW_3000'] : "http://localhost:3000";
console.log("dev server listening at: " + devHost);
app.listen(3000);
EOF