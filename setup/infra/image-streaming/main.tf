data "google_container_cluster" "broker" {
  name     = "${var.name}-${var.region}"
  location = var.region
  project =  var.project_id
}
