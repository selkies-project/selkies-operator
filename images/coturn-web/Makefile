# Google LLC 2019
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

deploy:
	gcloud beta run deploy coturn-web \
	  --image gcr.io/cloud-solutions-images/gpu-accel-webrtc-coturn-web:latest \
	  --no-allow-unauthenticated \
	  --project $${PROJECT_ID?env not set} \
	  --set-env-vars=\
	EXTERNAL_IP=$${EXTERNAL_IP?env not set},\
	TURN_SHARED_SECRET=$${TURN_SHARED_SECRET?env not set}