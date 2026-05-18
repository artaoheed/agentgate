resource "google_bigquery_dataset" "agentgate" {
  dataset_id    = var.bq_dataset_id
  friendly_name = "AgentGate governance dataset"
  description   = "Holds governance events emitted by AgentGate."
  location      = var.bq_location

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

  # Note: partitioning is intentionally omitted. The pre-existing
  # table imported from earlier ad-hoc creation is unpartitioned, and
  # BigQuery can't convert an unpartitioned table in place. If we ever
  # destroy+recreate this table (e.g. schema rewrite), add:
  #   time_partitioning { type = "DAY" }
  # which uses ingestion time (the `timestamp` payload field is a
  # STRING, not a TIMESTAMP, so it can't be used as a partition column).

  labels = local.labels
}
