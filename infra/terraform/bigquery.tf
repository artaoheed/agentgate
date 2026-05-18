resource "google_bigquery_dataset" "agentgate" {
  dataset_id    = var.bq_dataset_id
  friendly_name = "AgentGate governance dataset"
  description   = "Holds governance events emitted by AgentGate."
  location      = var.region

  labels = local.labels

  depends_on = [google_project_service.apis]
}

resource "google_bigquery_table" "governance_events" {
  dataset_id          = google_bigquery_dataset.agentgate.dataset_id
  table_id            = var.bq_table_id
  deletion_protection = false

  # Schema lives alongside the application code so it stays in sync
  # with the GovernanceEvent struct serialized by the Go service.
  schema = file("${path.module}/../../governance_events_schema.json")

  time_partitioning {
    type  = "DAY"
    field = "timestamp"
  }

  labels = local.labels
}
