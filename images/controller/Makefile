APPS_JSON := test/var/run/buildsrc/apps.json
BROKER_ENV := test/env
BUILD_SRC_DIR := $(dir $(APPS_JSON))
BUILD_DIR := test/var/run/build
BUILD_USR_DIR := test/var/run/buildusr
BROKER_ENV := test/env
USER_ID := $(shell id -u)
GROUP_ID := $(shell id -g)

build:
	docker build -t selkies-controller:latest .

$(BUILD_DIR):
	mkdir -p $@

$(BUILD_SRC_DIR):
	mkdir -p $@

$(BUILD_USR_DIR):
	mkdir -p $@

$(BROKER_ENV):
	mkdir -p $(dir $@)
	kubectl -n pod-broker-system get cm pod-broker-config -o json | jq -r '.data | to_entries[] | [.key, .value] | join("=")' > $@
	kubectl -n pod-broker-system get secret oauth-client-id -o json | jq -r '.data.CLIENT_ID' | base64 -d | xargs printf "OAUTH_CLIENT_ID=%s\n" >> $@
	kubectl -n pod-broker-system get secret pod-broker -o json | jq -r '.data.COOKIE_SECRET' | base64 -d | xargs printf "COOKIE_SECRET=%s\n" >> $@
	gcloud config get-value project 2>/dev/null | xargs printf "PROJECT_ID=%s\n" >> $@

.PRECIOUS: $(APPS_JSON)
$(APPS_JSON): $(BROKER_ENV) $(BUILD_SRC_DIR)
	kubectl version >/dev/null && \
	docker run -it --rm --entrypoint=app-finder \
		-u $(USER_ID):$(GROUP_ID) \
		--env-file=test/env \
		-e NAMESPACE=pod-broker-system \
		-e SINGLE_ITERATION=true \
		-e KUBECONFIG=/etc/kubeconfig \
		-v $(abspath $(BUILD_SRC_DIR)):/var/run/buildsrc \
		-v $$HOME/.kube/config:/etc/kubeconfig \
	selkies-controller:latest

.PHONY: fetch
fetch: clean-json $(APPS_JSON)

test-%: $(APPS_JSON) $(BUILD_DIR) $(BUILD_USR_DIR)
	kubectl version >/dev/null && \
	docker run -it --rm --entrypoint=$* \
		-u $(USER_ID):$(GROUP_ID) \
		--env-file=test/env \
		-e NAMESPACE=pod-broker-system \
		-e KUBECONFIG=/etc/kubeconfig \
		-v $$HOME/.kube/config:/etc/kubeconfig \
		-v $(abspath $(BUILD_DIR)):/var/run/build \
		-v $(abspath $(BUILD_SRC_DIR)):/var/run/buildsrc \
		-v $(abspath $(BUILD_USR_DIR)):/var/run/buildusr \
		-p 8082:8082 \
		-p 8081:8081 \
	selkies-controller:latest

.PHONY: clean
clean-json:
	rm -f $(APPS_JSON)
clean:
	rm -rf $(BUILD_SRC_DIR) $(BUILD_DIR) $(BUILD_USR_DIR)
