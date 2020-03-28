# Deploying the app launcher platform to a GCP project

This tutorial will walk you through deploying the kube-pod-broker platform with Terraform and Cloud Build.

## Setup

1. Set the project, replace `YOUR_PROJECT` with your project ID:

```bash
PROJECT=YOUR_PROJECT
```

```bash
gcloud config set project ${PROJECT}
```

## Enable APIs

1. Enable the services used by this tutorial:

```bash
gcloud services enable \
    compute.googleapis.com \
    container.googleapis.com \
    cloudbuild.googleapis.com \
    servicemanagement.googleapis.com \
    serviceusage.googleapis.com \
    secretmanager.googleapis.com
```

2. Grant the cloud build service account permissions on your project:

```bash
CLOUDBUILD_SA=$(gcloud projects describe $PROJECT --format='value(projectNumber)')@cloudbuild.gserviceaccount.com && \
  gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$CLOUDBUILD_SA --role roles/owner
```

## Build images

1. Build the images using cloud build:

```bash
(cd images && gcloud builds submit --config cloudbuild.yaml)
```

## OAuth Task 1/2 - Configure OAuth consent screen

1. Go to the [OAuth consent screen](https://console.cloud.google.com/apis/credentials/consent). If prompted for application type, select __External__.
2. Under __Application name__, enter `App Launcher`.
3. Under __Support email__, select the email address you want to display as a public contact. This must be your email address, or a Google Group you own.
4. Add any optional details you’d like.
5. Click __Save__.

## OAuth Task 2/2 - Create OAuth credentials

1. Go to the [Credentials page](https://console.cloud.google.com/apis/credentials)
2. Click __Create Credentials > OAuth client ID__,
3. Under __Application type__, select __Web application__. In the __Name__ box, enter `App Launcher`
4. When you are finished, click __Create__. After your credentials are created, make note of the client ID and client secret that appear in the OAuth client window.
5. In Cloud Shell, save your OAuth credentials obtained to variables:

```bash
export CLIENT_ID=YOUR_CLIENT_ID
```

```bash
export CLIENT_SECRET=YOUR_CLIENT_SECRET
```

6. Store the OAuth credentials in Secret Manager:

```bash
gcloud secrets create broker-oauth2-client-id --replication-policy=automatic --data-file <(echo -n ${CLIENT_ID})
```

```bash
gcloud secrets create broker-oauth2-client-secret --replication-policy=automatic --data-file <(echo -n ${CLIENT_SECRET})
```

6. In the Cloud Console, go back to the credential you just created and edit the __Authorized redirect URIs__, add the URL from the output below and then press __enter__ to add the entry to the list.

```bash
echo "https://iap.googleapis.com/v1/oauth/clientIds/${CLIENT_ID}:handleRedirect"
```

6. Click __save__

## Generate pod broker cookie secret

1. Generate a secret used to create and verify pod broker cookies:

```bash
export COOKIE_SECRET=$(openssl rand -base64 15)
```

2. Store the cookie secret in Secret Manager:

```bash
gcloud secrets create broker-cookie-secret --replication-policy=automatic --data-file <(echo -n ${COOKIE_SECRET})
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
(cd setup/infra/cluster && gcloud builds submit --substitutions=_REGION=${REGION})
```

> NOTE: this can be run multiple times with different regions.

3. Create the workload identity bindinds:

```bash
(cd setup/infra/wi-sa && gcloud builds submit)
```

> NOTE: this is a workaround because the identity namespace does not exist until the first cluster in a project has been created with worload identity enabled.

4. Deploy the manifests to the regional cluster:

```bash
(cd setup/manifests && gcloud builds submit --substitutions=_REGION=${REGION})
```

> NOTE: this can be run multiple times with different regions.

## Connect to the web interface

1. Add your current user to the IAP authorized web users role:

```bash
./setup/scripts/add_iap_user.sh user $(gcloud config get-value account) ${PROJECT}
```

2. Wait for the global load balancer and managed certificates to be provisioned.

3. Open the web interface:

```bash
echo "Open: https://broker.endpoints.$(gcloud config get-value project 2>/dev/null).cloud.goog/"
```

> NOTE: it may take several seconds for the IAM permissions to propagate, during this time you may see an Access Denied page.

> NOTE: at this point there will be no apps listed.

## Deploy the sample remote application

1. Deploy the sample app using Cloud Build:

```bash
(cd examples/jupyter-notebook/ && gcloud builds submit --substitutions=_REGION=${REGION})
```

2. Refresh the app launcher interface to launch the app.
