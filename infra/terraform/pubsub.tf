resource "google_pubsub_topic" "governance_events" {
  name = var.pubsub_topic_name

  labels = local.labels

  depends_on = [google_project_service.apis]
}

# The Pub/Sub service agent (created by GCP when the API is enabled)
# needs write access on the BigQuery table for a BigQuery subscription
# to deliver messages.
resource "google_project_iam_member" "pubsub_bq_writer" {
  project = var.project_id
  role    = "roles/bigquery.dataEditor"
  member  = "serviceAccount:service-${local.project_number}@gcp-sa-pubsub.iam.gserviceaccount.com"

  depends_on = [google_project_service.apis]
}

resource "google_project_iam_member" "pubsub_bq_metadata_viewer" {
  project = var.project_id
  role    = "roles/bigquery.metadataViewer"
  member  = "serviceAccount:service-${local.project_number}@gcp-sa-pubsub.iam.gserviceaccount.com"

  depends_on = [google_project_service.apis]
}

resource "google_pubsub_subscription" "bq_sink" {
  name  = "${var.pubsub_topic_name}-to-bq"
  topic = google_pubsub_topic.governance_events.id

  bigquery_config {
    table            = "${var.project_id}.${google_bigquery_dataset.agentgate.dataset_id}.${google_bigquery_table.governance_events.table_id}"
    write_metadata   = false
    use_topic_schema = false
  }

  # Ack deadline matters less for a BigQuery subscription (Pub/Sub
  # writes synchronously) but the default is still applied.
  ack_deadline_seconds = 60

  depends_on = [
    google_project_iam_member.pubsub_bq_writer,
    google_project_iam_member.pubsub_bq_metadata_viewer,
  ]
}
