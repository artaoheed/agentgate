provider "google" {
  project = var.project_id
  region  = var.region
}

data "google_project" "this" {}

locals {
  # Used to construct service-agent emails for built-in GCP services
  # (e.g. the Pub/Sub agent that needs BigQuery write permission).
  project_number = data.google_project.this.number

  # Common labels applied to every resource that supports them.
  labels = {
    app        = "agentgate"
    managed_by = "terraform"
    env        = var.env
  }
}
