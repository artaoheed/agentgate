resource "google_artifact_registry_repository" "containers" {
  repository_id = var.artifact_repo_id
  location      = var.region
  format        = "DOCKER"
  description   = "Container images for AgentGate."

  labels = local.labels

  depends_on = [google_project_service.apis]
}
