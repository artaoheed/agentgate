# ---------------------------------------------------------------------
# Runtime service account: identity used by the Cloud Run container.
# Granted the minimum needed to publish governance events and read the
# Gemini API key.
# ---------------------------------------------------------------------

resource "google_service_account" "runtime" {
  account_id   = "${var.service_name}-runtime"
  display_name = "AgentGate Cloud Run runtime"
  description  = "Identity for the AgentGate Cloud Run service."

  depends_on = [google_project_service.apis]
}

resource "google_pubsub_topic_iam_member" "runtime_publisher" {
  topic  = google_pubsub_topic.governance_events.name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_secret_manager_secret_iam_member" "runtime_secret_accessor" {
  secret_id = google_secret_manager_secret.gemini_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime.email}"
}

# ---------------------------------------------------------------------
# CI/CD service account: impersonated by GitHub Actions via WIF to
# build images and deploy revisions.
# ---------------------------------------------------------------------

resource "google_service_account" "cicd" {
  account_id   = "${var.service_name}-cicd"
  display_name = "AgentGate CI/CD"
  description  = "Identity impersonated by GitHub Actions to deploy AgentGate."

  depends_on = [google_project_service.apis]
}

resource "google_artifact_registry_repository_iam_member" "cicd_writer" {
  location   = google_artifact_registry_repository.containers.location
  repository = google_artifact_registry_repository.containers.name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.cicd.email}"
}

resource "google_project_iam_member" "cicd_run_developer" {
  project = var.project_id
  role    = "roles/run.developer"
  member  = "serviceAccount:${google_service_account.cicd.email}"
}

# Cloud Run deploy needs actAs on the runtime SA so the new revision
# can run as that identity.
resource "google_service_account_iam_member" "cicd_actas_runtime" {
  service_account_id = google_service_account.runtime.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.cicd.email}"
}
