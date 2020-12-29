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

FROM nginx:alpine

# Copy web assets
WORKDIR /usr/share/nginx/html/
COPY src/app.js src/index.html ./
COPY src/pod_broker.js .
COPY config/default.conf /etc/nginx/conf.d/default.conf

# Copy CDN resources
WORKDIR /usr/share/nginx/html/vendor/

ADD "https://cdnjs.cloudflare.com/ajax/libs/vuetify/2.1.12/vuetify.min.css" vuetify.min.css
ADD "https://cdnjs.cloudflare.com/ajax/libs/vue/2.6.9/vue.min.js" vue.min.js
ADD "https://cdnjs.cloudflare.com/ajax/libs/vuetify/2.1.12/vuetify.min.js" vuetify.min.js
ADD "https://cdn.jsdelivr.net/npm/vue-spinner@1.0.3/dist/vue-spinner.min.js" vue-spinner.min.js
RUN chmod 0644 *

WORKDIR /usr/share/nginx/html/

# Patch index.html to fetch latest version of javascript source
RUN sed -i 's|script src="\(.*\)?ts=.*"|script src="\1?ts='$(date +%s)'"|g' index.html