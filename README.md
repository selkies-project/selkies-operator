# Kubernetes App Launcher

Operator for orchestrating per-user stateful workloads.

## Quick start

1. Enable the required GCP project services:

```bash
gcloud services enable --project ${PROJECT_ID} \
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

3. Deploy with Cloud Build:

```bash
ACCOUNT=$(gcloud config get-value account)
REGION=us-west1

gcloud builds submit --project=${PROJECT_ID} --substitutions=_USER=${ACCOUNT},_REGION=${REGION}
```

4. Deploy sample app:

```bash
(cd examples/jupyter-notebook/ && gcloud builds submit --project=${PROJECT_ID} --substitutions=_REGION=${REGION})
```

5. Connect to the App Launcher web interface at the URL output below:

```bash
# Print real URL
echo "https://broker.endpoints.${PROJECT_ID}.cloud.goog/"
```
