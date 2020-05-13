# Deploying the app launcher platform to a GCP project

This tutorial will walk you through deploying the kube-pod-broker platform with Terraform and Cloud Build.

## Setup

1. Set the project, replace `YOUR_PROJECT` with your project ID:

```bash
export PROJECT_ID=YOUR_PROJECT
```

```bash
gcloud config set project ${PROJECT_ID?}
```

## Enable APIs

1. Enable the services used by this tutorial:

```bash
gcloud services enable --project ${PROJECT_ID?} \
    cloudresourcemanager.googleapis.com \
    compute.googleapis.com \
    container.googleapis.com \
    cloudbuild.googleapis.com \
    servicemanagement.googleapis.com \
    serviceusage.googleapis.com \
    stackdriver.googleapis.com \
    secretmanager.googleapis.com \
    iap.googleapis.com
```

2. Grant the cloud build service account permissions on your project:

```bash
CLOUDBUILD_SA=$(gcloud projects describe ${PROJECT_ID?} --format='value(projectNumber)')@cloudbuild.gserviceaccount.com && \
  gcloud projects add-iam-policy-binding ${PROJECT_ID?} --member serviceAccount:${CLOUDBUILD_SA?} --role roles/owner && \
  gcloud projects add-iam-policy-binding ${PROJECT_ID?} --member serviceAccount:${CLOUDBUILD_SA?} --role roles/iam.serviceAccountTokenCreator
```

## Build images

1. Build the images using cloud build:

```bash
(cd images && gcloud builds submit --config cloudbuild.yaml)
```

## Create OAuth Client

1. Create the OAuth brand and client:

```bash
eval $(./setup/scripts/create_oauth_client.sh "App Launcher")
```

> NOTE: this command automatically creates the OAuth client ID and secret and exports them to variables CLIENT_ID and CLIENT_SECRET in your shell.

> NOTE: if you want to authorize users outside of your organization, you must click the __MAKE EXTERNAL__ button from the [OAuth Consent Screen page](https://console.cloud.google.com/apis/credentials/consent?project=disla-vdi-demo) in the cloud console.

2. Store the OAuth credentials in Secret Manager:

```bash
gcloud secrets create broker-oauth2-client-id --replication-policy=automatic --data-file <(echo -n ${CLIENT_ID?})
```

```bash
gcloud secrets create broker-oauth2-client-secret --replication-policy=automatic --data-file <(echo -n ${CLIENT_SECRET?})
```

## Generate pod broker cookie secret

1. Generate a secret used to create and verify pod broker cookies:

```bash
export COOKIE_SECRET=$(openssl rand -base64 15)
```

2. Store the cookie secret in Secret Manager:

```bash
gcloud secrets create broker-cookie-secret --replication-policy=automatic --data-file <(echo -n ${COOKIE_SECRET?})
```

## Deploy the infrastructure

1. Deploy the base infrastructure with Cloud Build:

```bash
(cd setup && gcloud builds submit)
```

2. Deploy the cluster for your desired region:

```bash
REGION=us-west1
```

```bash
(cd setup/infra/cluster && gcloud builds submit --substitutions=_REGION=${REGION?})
```

3. Create the node pool for apps

```bash
(cd setup/infra/node-pool-apps && gcloud builds submit --substitutions=_REGION=${REGION?})
```

4. Create the node pool for GPU accelerated apps

```bash
(cd setup/infra/node-pool-gpu && gcloud builds submit --substitutions=_REGION=${REGION?})
```

5. Create the workload identity bindings:

```bash
(cd setup/infra/wi-sa && gcloud builds submit)
```

> NOTE: this is a workaround because the identity namespace does not exist until the first cluster in a project has been created with worload identity enabled.

6. Deploy the manifests to the regional cluster:

```bash
(cd setup/manifests && gcloud builds submit --substitutions=_REGION=${REGION?})
```

> NOTE: this can be run multiple times with different regions.

## Connect to the web interface

1. Add your current user to the IAP authorized web users role:

```bash
./setup/scripts/add_iap_user.sh user $(gcloud config get-value account) ${PROJECT_ID?}
```

2. Wait for the global load balancer and managed certificates to be provisioned.

3. Open the web interface:

```bash
echo "Open: https://broker.endpoints.$(gcloud config get-value project 2>/dev/null).cloud.goog/"
```

> NOTE: it may take several seconds for the IAM permissions to propagate, during this time you may see an Access Denied page.

> NOTE: at this point there will be no apps listed.

## Create the node pool for apps

1. Create the `tier1` node pool for apps:

```bash
(cd setup/infra/node-pool-apps && gcloud builds submit)
```

2. Deploy the sample app using Cloud Build:

```bash
(cd examples/jupyter-notebook/ && gcloud builds submit --substitutions=_REGION=${REGION?})
```

3. Refresh the app launcher interface to launch the app.
