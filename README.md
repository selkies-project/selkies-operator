# Selkies - Stateful Workload Operator

[![Discord](https://img.shields.io/discord/798699922223398942?logo=discord)](https://discord.gg/wDNGDeSW5F)

Selkies is a platform built on GKE to orchestrate per-user stateful workloads.

## Quick start

### Assumptions

* You are a member of a Google Cloud [organization](https://cloud.google.com/resource-manager/docs/creating-managing-organization).
  * This is required for `setup/scripts/create_oauth_client.sh` to use `gcloud alpha iap oauth-brand` commands, because these implicity operate on organization-internal brands. For more information, see [this guide](https://cloud.google.com/iap/docs/programmatic-oauth-clients).
* You are granted the `Owner` role in a project in that organization. <!-- TODO: Can we de-escalate this role assumption? What minimal role/permissions are actually required? -->
* You have `gcloud` [installed](https://cloud.google.com/sdk/docs/install) in your environment.

### Steps

The steps below will create the infrastructure for the app launcher. You should deploy to a new project.

1. Clone the source repository:

    ```bash
    git clone -b master https://github.com/selkies-project/selkies.git
    cd selkies
    ```

1. Configure gcloud (replace `XXX` & `us-west1` with your project ID & preferred region): <!-- TODO: Add a "supported regions" list? What regions can/can't Selkies run in? -->

    ```bash
    export PROJECT_ID=XXX
    export REGION=us-west1
    gcloud config set project ${PROJECT_ID?}
    gcloud config set compute/region ${REGION?}
    ```

1. Enable the required GCP project services:

    ```bash
    gcloud services enable \
        --project ${PROJECT_ID?} \
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

1. Grant the cloud build service account permissions on your project:

    ```bash
    PROJECT_NUMBER=$(
      gcloud projects describe ${PROJECT_ID?} \
        --format='value(projectNumber)'
    ) && \
      CLOUDBUILD_SA="${PROJECT_NUMBER?}@cloudbuild.gserviceaccount.com" && \
      gcloud projects add-iam-policy-binding ${PROJECT_ID?} \
        --member serviceAccount:${CLOUDBUILD_SA?} \
        --role roles/owner && \
      gcloud projects add-iam-policy-binding ${PROJECT_ID?} \
        --member serviceAccount:${CLOUDBUILD_SA?} \
        --role roles/iam.serviceAccountTokenCreator
    ```

1. Deploy with Cloud Build:

    ```bash
    ACCOUNT=$(gcloud config get-value account) && \
      gcloud builds submit \
        --project=${PROJECT_ID?} \
        --substitutions=_USER=${ACCOUNT?},_REGION=${REGION?}
    ```

1. Deploy sample app:

    ```bash
    (cd examples/jupyter-notebook/ && \
      gcloud builds submit \
        --project=${PROJECT_ID?} \
        --substitutions=_REGION=${REGION?})
    ```

1. Connect to the App Launcher web interface at the URL output below:

    ```bash
    echo "https://broker.endpoints.${PROJECT_ID?}.cloud.goog/"
    ```

### Troubleshooting

* If the initial cloud build fails with the message `Step #2 - "create-oauth-client": ERROR: (gcloud.alpha.iap.oauth-brands.list) INVALID_ARGUMENT: Request contains an invalid argument.` It is most likely due to running as a user that is not a member of the Cloud Identity Organization. See the limitation described above.

* If your region only has 500 GB of Persistent Disk SSD quota, run the following but keep in mind the number of apps and image pull performance will be affected.

    ```bash
    cat - > selkies-min-ssd.auto.tfvars <<EOF
    default_pool_disk_size_gb = 100
    turn_pool_disk_size_gb = 100
    gpu_cos_pool_disk_size_gb = 100
    tier1_pool_disk_size_gb = 100
    EOF
    ```

    ```bash
    gcloud secrets create broker-tfvars-selkies-min-ssd \
        --replication-policy=automatic \
        --data-file selkies-min-ssd.auto.tfvars
    ```

* If the load balancer never comes online and you receive 500 errors after the deployment has completed for at least 30 minutes, the autoneg controller annotation may need to be reset:

    ```bash
    gcloud container clusters get-credentials broker-${REGION?}
    ```

    ```bash
    ./setup/scripts/fix_autoneg.sh
    ```
