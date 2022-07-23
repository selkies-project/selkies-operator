data "google_container_cluster" "broker" {
  name     = "${var.name}-${var.region}"
  location = var.region
  project =  var.project_id
}

data "google_service_account" "broker_cluster" {
  account_id = var.name
  project =  var.project_id
}
