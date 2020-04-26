/*
 Copyright 2019 Google Inc. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

const http = require('http')
httpProxy = require('http-proxy')
url = require('url');

const getEnvOrDie = (name) => {
    if (name in process.env) {
        return process.env[name];
    }
    console.log(`Missing env ${name}`);
    process.exit();
}

// Get required env vars.
var CLIENT_ID = getEnvOrDie("CLIENT_ID");
var BROKER_ENDPOINT = getEnvOrDie("BROKER_ENDPOINT");
var BROKER_COOKIE = getEnvOrDie("BROKER_COOKIE");

// Create the proxy server.
var proxy = httpProxy.createProxyServer({
    target: {
        protocol: 'https:',
        host: url.parse(BROKER_ENDPOINT).host,
        port: 443
    },
    changeOrigin: true,
    ws: true,
});

// Function to fetch identity token from GCE metadata server.
var getToken = (token) => {
    return new Promise((resolve, reject) => {
        var refresh = false;
        if (token === "") {
            refresh = true;
        }
        let toks = token.split(".");
        if (toks.length === 3) {
            let payload = JSON.parse(Buffer.from(toks[1], "base64").toString());
            let expTime = payload.exp - Math.floor(new Date().getTime() / 1000);
            if (expTime < 0) {
                // Refresh if token is expired.
                refresh = true;
            }
        } else {
            // refresh if token is bad.
            refresh = true;
        }
        if (refresh === true) {
            console.log("INFO: Refreshing token");

            const options = {
                hostname: 'metadata.google.internal',
                port: 80,
                path: '/computeMetadata/v1/instance/service-accounts/default/identity?audience=' + CLIENT_ID + '&format=full',
                method: 'GET',
                headers: {
                    'Metadata-Flavor': 'Google'
                }
            }

            const req = http.get(options, (res) => {
                if (res.statusCode !== 200) {
                    console.log("ERROR: Failed to fetch token, status: " + res.statusCode);
                    reject();
                } else {
                    res.on('data', (d) => {
                        resolve(d.toString());
                    });
                }
            });
        } else {
            resolve(token);
        }
    });
}

// Token variable used by proxy function.
var token = "";

// Proxy request handler to inject identity token and broker cookie.
proxy.on('proxyReq', function (proxyReq, req, res, options) {
    proxyReq.setHeader('Authorization', 'Bearer ' + token);
    proxyReq.setHeader('Cookie', BROKER_COOKIE);
});

console.log("INFO: listening on port 5050")
http.createServer((req, res) => {
    getToken(token).then((t) => {
        token = t;
        proxy.web(req, res);
    });
}).listen(5050);
