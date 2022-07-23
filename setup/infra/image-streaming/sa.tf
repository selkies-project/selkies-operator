resource "google_project_iam_member" "cluster_service_account-artifact-registry" {
  project = var.project_id
  role     = "roles/artifactregistry.reader"
  member   = "serviceAccount:${data.google_service_account.broker_cluster.email}"
}

resource "google_project_service" "containerfilesystem" {
  for_each = toset(var.gcp_service_list)
  
  project =  var.project_id
  service = each.key

  timeouts {
    create = "30m"
    update = "40m"
  }

  disable_dependent_services = true
}