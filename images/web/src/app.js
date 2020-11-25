/**
 * Copyright 2019 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

var brokerURL = window.location.origin + window.location.pathname + "broker/";
var reservationBrokerURL = window.location.origin + window.location.pathname + "reservation-broker/";

var podBroker = new PodBroker(brokerURL);
var reservationPodBroker = new PodBroker(reservationBrokerURL);

function getStorageItem(appName, key, defaultValue) {
    return window.localStorage.getItem(appName + "." + key) || defaultValue;
}

function setStorageItem(appName, key, value) {
    window.localStorage.setItem(appName + "." + key, value);
}

function getParameterByName(name, url = window.location.href) {
    name = name.replace(/[\[\]]/g, '\\$&');
    var regex = new RegExp('[?&]' + name + '(=([^&#]*)|&|#|$)'),
        results = regex.exec(url);
    if (!results) return null;
    if (!results[2]) return '';
    return decodeURIComponent(results[2].replace(/\+/g, ' '));
}

class BrokerApp {
    constructor(name, type, displayName, description, icon, launchURL, defaultRepo, defaultTag, params, defaultNodeTiers, disableOptions) {
        this.broker = null;

        this.name = name;
        this.type = type;
        this.displayName = displayName;
        this.description = description;
        this.icon = icon;
        this.disableOptions = disableOptions;
        this.status = "checking";
        this.saveStatus = "idle";
        this.saveError = "";
        this.launchURL = launchURL;
        this.waitLaunch = false;

        this.params = params;
        this.paramValues = {};
        this.params.forEach((item) => {
            if (item.type == "bool") {
                this.paramValues[item.name] = (item.defaultValue === "true");
            }
        });

        this.defaultRepo = defaultRepo;
        this.defaultTag = defaultTag;
        this.imageRepo = (getStorageItem(this.name, "imageRepo", ""));
        this.imageTag = (getStorageItem(this.name, "imageTag", "latest"))
        this.imageTags = [];

        this.nodeTiers = defaultNodeTiers;
        this.nodeTier = "";

        this.checkAppTotal = 1;
        this.checkAppCount = 0;
    }

    setParamValues(values) {
        this.params.forEach((item) => {
            if (item.type == "bool") {
                this.paramValues[item.name] = (values[item.name] === "true");
            }
        });
    }

    getLaunchParams() {
        return {}
    }

    launch() {
        if (this.status === "ready") {
            window.location.href = this.launchURL;
        } else {
            this.status = "checking";
            // Build launch params.
            this.waitLaunch = true;
            var launchParams = this.getLaunchParams();
            this.broker.start_app(this.name, launchParams, () => {
                this.update(true);
            });
        }
    }

    shutdown() {
        if (this.status === "stopped") return;
        this.status = "terminating";
        this.broker.shutdown_app(this.name, () => {
            this.status = "terminating";
            setTimeout(() => {
                this.update(true);
            }, 3000);
        });
    }

    update(loop) {
        this.broker.get_status(this.name, (data) => {
            switch (data.status) {
                case "ready":
                    this.status = "running";
                    this.checkApp(loop);
                    break;
                case "shutdown":
                    this.status = "stopped";
                    break;
                default:
                    this.status = "checking";
                    break;
            }
        },
            () => {
                if (loop && this.status === "checking") {
                    setTimeout(() => {
                        this.update(loop);
                    }, 2000);
                }
            });
    }

    checkApp(loop) {
        console.log(this.name + " consecutive health check count: " + this.checkAppCount + "/" + this.checkAppTotal);
        fetch(this.launchURL, {
            mode: 'no-cors',
            cache: 'no-cache',
            credentials: 'include',
            redirect: 'follow',
        })
            .then((response) => {
                if (response.status < 400) {
                    if (this.checkAppCount++ >= this.checkAppTotal) {
                        this.status = "ready";
                        this.checkAppCount = 0;

                        if (this.waitLaunch) {
                            this.waitLaunch = false;
                            window.location.href = this.launchURL;
                        }
                    }
                } else {
                    // reset check
                    this.checkAppCount = 0;
                }
            })
            .catch((err) => {
                this.checkAppCount = 0;
            })
            .finally(() => {
                if (loop && this.status === "running") {
                    setTimeout(() => {
                        this.checkApp(loop);
                    }, 1000);
                }
            });
    }

    saveItem(key, val) {
        setStorageItem(this.name, key, val);
    }

    saveConfig() {
        this.saveStatus = "saving";
        var data = {
            "imageRepo": this.imageRepo,
            "imageTag": this.imageTag,
            "nodeTier": this.nodeTier,
            "params": {},
        }
        Object.keys(this.paramValues).forEach(key => data.params[key] = this.paramValues[key].toString());
        this.broker.set_config(this.name, data, (resp) => {
            if (resp.code !== 200) {
                this.saveStatus = "failed";
                this.saveError = resp.status;
            } else {
                this.saveStatus = "saved";
                setTimeout(() => {
                    this.saveStatus = "idle";
                }, 1500)
            }
        });
    }

    fetchConfig() {
        if (this.type !== "statefulset") return;
        this.saveStatus = "idle";
        this.broker.get_config(this.name, (data) => {
            this.imageRepo = data.imageRepo;
            this.imageTag = data.imageTag;
            this.imageTags = data.tags;
            this.nodeTier = data.nodeTier;
            this.setParamValues(data.params);
        });
    }
}

var ScaleLoader = VueSpinner.ScaleLoader;
var vue_app = new Vue({
    el: '#app',
    components: {
        ScaleLoader
    },
    vuetify: new Vuetify(),
    created() {
        this.$vuetify.theme.dark = true
    },
    data() {
        return {
            brokerName: "App Launcher",
            brokerRegion: "",
            darkTheme: false,
            quickLaunchEnabled: false,

            // array of BrokerApp objects.
            apps: [],

            launchDisabled: (app) => {
                var appReady = (['stopped', 'ready'].indexOf(app.status) < 0);
            },

            getQuickLaunchApp: () => {
                return getParameterByName("launch");
            },

            checkQuickLaunch: () => {
                var curr_app = this.getQuickLaunchApp();
                if (curr_app === null) return;

                var found = false;
                this.apps.forEach((app) => {
                    if (app.name === curr_app) {
                        this.quickLaunchEnabled = true;
                        found = true;
                        console.log("launching app: " + curr_app);
                        app.launch();
                    }
                });

                if (found === false) {
                    this.quickLaunchEnabled = false;
                    console.log("WARN: quick launch app not found: " + curr_app);
                }
            }
        }
    },

    computed: {
        sortedApps: function () {
            function compare(a, b) {
                if (a.name < b.name)
                    return -1;
                if (a.name > b.name)
                    return 1;
                return 0;
            }

            return this.apps.sort(compare);
        }
    },

    watch: {
        brokerName: (val) => {
            document.title = val;
        },
    },
});

var fetchApps = () => {
    // Fetch list of apps.
    podBroker.get_apps((data) => {
        vue_app.brokerName = data.brokerName;
        vue_app.brokerRegion = data.brokerRegion;
        vue_app.$vuetify.theme.dark = data.brokerTheme === "dark";

        // Fetch app status.
        data.apps.forEach((item) => {
            var app = new BrokerApp(
                item.name,
                item.type,
                item.displayName,
                item.description,
                item.icon,
                item.launchURL,
                item.defaultRepo,
                item.defaultTag,
                item.params,
                item.nodeTiers,
                item.disableOptions
            );
            // Set broker based on app type.
            app.broker = (item.type === "deployment") ? reservationPodBroker : podBroker;

            vue_app.apps.push(app);
            app.update(true);

            // Fetch user app config
            app.fetchConfig();
        });

        // Check to see if launch app was passed in route.
        vue_app.checkQuickLaunch();
    });    
}

// If launch app was provided via route, skip the app list and show loading.
vue_app.quickLaunchEnabled = (vue_app.getQuickLaunchApp() !== null);

fetchApps();