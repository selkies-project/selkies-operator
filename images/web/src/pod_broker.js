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

class PodBroker {
    /**
     * Provides API bindings for Pod Broker
     * 
     * @constructor
     * @param {String} [brokerURL]
     *    The endpoint to connect to.
     */
    constructor(broker_url) {
        this.broker_url = broker_url;
    }

    /**
     * Retrieves list of apps from broker.
     * 
     * @param {Function} cb The callback function accepting the data object.
     */
    get_apps(cb) {
        fetch(this.broker_url)
            .then(function (response) {
                return response.json();
            })
            .then(cb);
    }

    /**
     * Retrieves app status for named app.
     * 
     * @param {String} app_name The name of the app to get.
     * @param {Function} cb The callback function accepting the data object.
     * @param {Function} fcb The callback function that runs after fetch promise resolves after any completion.
     */
    get_status(app_name, cb, fcb) {
        fetch(this.broker_url + app_name + "/", { credentials: 'include' })
            .then(function (response) {
                console.log(`${app_name} status: ${response.status}`);
                return response.json();
            })
            .then(cb)
            .finally(fcb);
    }

    /**
     * Starts named app
     * 
     * @param {String} app_name The name of the app to start.
     * @param {Object} params object of parameters to pass.
     * @param {Function} cb The callback function accepting the data object.
     */
    start_app(app_name, params, cb) {
        var url = new URL(this.broker_url + app_name + "/");
        Object.keys(params).forEach(key => url.searchParams.append(key, params[key]));
        fetch(url, { method: "POST" })
            .then(cb);
    }

    /**
     * Shutdown the named app
     * 
     * @param {String} app_name The name of the app to shutdown.
     * @param {Function} cb The callback function accepting the data object.
     */
    shutdown_app(app_name, cb) {
        fetch(this.broker_url + app_name + "/", { method: "DELETE" })
            .then(cb);
    }

    /**
     * 
     * @param {String} app_name The name of the app to set the image repo for
     * @param {Object} data object of config spec to pass.
     * @param {Function} cb The callback function accepting the data object.
     * @param {Function} fcb The callback function that runs after fetch promise resolves after any completion.
     */
    set_config(app_name, data, cb, fcb) {
        var url = new URL(this.broker_url + app_name + "/config/");
        fetch(url, {
            method: "POST",
            headers: {
                "content-type": "application/json"
            },
            body: JSON.stringify(data),
        })
            .then(function (response) {
                return response.json();
            })
            .then(cb)
            .finally(fcb);
    }

    /**
     * 
     * @param {String} app_name The name of the app to set the image repo for
     * @param {Function} cb The callback function accepting the data object.
     * @param {Function} fcb The callback function that runs after fetch promise resolves after any completion.
     */
    get_config(app_name, cb, fcb) {
        var url = new URL(this.broker_url + app_name + "/config/");
        fetch(url, { method: "GET", credentials: 'include' })
            .then(function (response) {
                return response.json();
            })
            .then(cb)
            .finally(fcb);
    }
};