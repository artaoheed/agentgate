output "cloud_run_url" {
  description = "Public URL of the Cloud Run service."
  value       = google_cloud_run_v2_service.agentgate.uri
}

output "runtime_service_account" {
  description = "Email of the runtime service account used by Cloud Run."
  value       = google_service_account.runtime.email
}

output "cicd_service_account" {
  description = "Email of the CI/CD service account impersonated by GitHub Actions."
  value       = google_service_account.cicd.email
}

output "wif_provider" {
  description = "Full resource name of the WIF provider (set as GITHUB_WIF_PROVIDER secret)."
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "artifact_registry_repo" {
  description = "Artifact Registry repo path for docker push."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.containers.name}"
}

output "pubsub_topic" {
  description = "Pub/Sub topic name for governance events."
  value       = google_pubsub_topic.governance_events.name
}

output "bigquery_table" {
  description = "Fully-qualified BigQuery table receiving governance events."
  value       = "${var.project_id}.${google_bigquery_dataset.agentgate.dataset_id}.${google_bigquery_table.governance_events.table_id}"
}
