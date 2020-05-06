# Kubernetes App Launcher

Operator for orchestrating per-user stateful workloads.

## Limitations

* The instructions below must be run within a Google Cloud Organization by a member of that org. This is due to the use of  `setup/scripts/create_oauth_client.sh`'s use of `gcloud alpha iap oauth-brand` commands - which implicity operate on internal brands. For details see https://cloud.google.com/iap/docs/programmatic-oauth-clients

## Quick start

The steps below will create the infrastructure for the app launcher. You should deploy to a new project.

1. Clone the source repository:

```bash
git clone https://github.com/GoogleCloudPlatform/solutions-k8s-stateful-workload-operator.git -b v1.0.0 && \
  cd solutions-k8s-stateful-workload-operator
```

2. Set the project, replace `YOUR_PROJECT` with your project ID:

```bash
export PROJECT_ID=YOUR_PROJECT
```

```bash
gcloud config set project ${PROJECT_ID?}
```

3. Enable the required GCP project services:

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

4. Grant the cloud build service account permissions on your project:

```bash
CLOUDBUILD_SA=$(gcloud projects describe ${PROJECT_ID?} --format='value(projectNumber)')@cloudbuild.gserviceaccount.com && \
  gcloud projects add-iam-policy-binding ${PROJECT_ID?} --member serviceAccount:${CLOUDBUILD_SA?} --role roles/owner && \
  gcloud projects add-iam-policy-binding ${PROJECT_ID?} --member serviceAccount:${CLOUDBUILD_SA?} --role roles/iam.serviceAccountTokenCreator
```

5. Deploy with Cloud Build:

```bash
ACCOUNT=$(gcloud config get-value account)
REGION=us-west1

gcloud builds submit --project=${PROJECT_ID?} --substitutions=_USER=${ACCOUNT?},_REGION=${REGION?}
```

6. Deploy sample app:

```bash
(cd examples/jupyter-notebook/ && gcloud builds submit --project=${PROJECT_ID?} --substitutions=_REGION=${REGION?})
```

7. Connect to the App Launcher web interface at the URL output below:

```bash
# Print real URL
echo "https://broker.endpoints.${PROJECT_ID?}.cloud.goog/"
```

## Troubleshooting
* If the initial cloud build fails with the message `Step #2 - "create-oauth-client": ERROR: (gcloud.alpha.iap.oauth-brands.list) INVALID_ARGUMENT: Request contains an invalid argument.` It is most likely due to running as a user that is not a member of the Cloud Identity Organization. See the limitation described above.
