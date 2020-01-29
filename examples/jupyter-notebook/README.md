## Jupyter Notebook Broker App Example

This example shows how to deploy a per-user Jupyter Notebook.

## Setup

1. Clone the source repo and change to the examples directory:

```bash
gcloud source repos clone kube-pod-broker --project=cloud-ce-pso-shared-code
```

```bash
cd kube-pod-broker/examples
```

## Option 1 - Deployment with GitOps

1. Run script to create and configure gitops Cloud Source Repository for the app:

```bash
cd jupyter-notebook
```

```bash
./jupyter-notebook-gitops-init.sh
```

> Follow the prompts to create the repository and trigger the cloud build.

2. Wait for Cloud Build to complete.

3. Navigate to the broker web interface and launch the app.

4. Make any changes to the app and push them to the `gitops` remote to deploy them to the cluster.

5. If you want to deploy to clusters in other regions, edit the `cloudbuild.yaml` file.

## Option 2 - Deployment with kubectl

1. Deploy the manifests and BrokerAppConfig custom resource:

```bash
kubectl apply -k jupyter-notebook/
```

2. Navigate to the broker web interface and launch the app.