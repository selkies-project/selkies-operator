resource "google_artifact_registry_repository" "selkies-repo" {
  provider = google-beta
  location = var.region
  repository_id = "selkies-images"
  description = "selkies image artifact registry"
  format = "DOCKER"
}