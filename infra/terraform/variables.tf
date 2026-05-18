variable "project_id" {
  description = "GCP project ID hosting AgentGate."
  type        = string
}

variable "region" {
  description = "GCP region for regional resources (Cloud Run, Artifact Registry, BigQuery)."
  type        = string
  default     = "us-central1"
}

variable "env" {
  description = "Deployment environment label (prod, staging, dev)."
  type        = string
  default     = "prod"
}

variable "service_name" {
  description = "Cloud Run service name."
  type        = string
  default     = "agentgate"
}

variable "image" {
  description = "Container image for Cloud Run. Defaults to the GCP hello image so the service can be created before CI/CD has pushed a real image. CI/CD updates this out-of-band; lifecycle.ignore_changes keeps Terraform from reverting it."
  type        = string
  default     = "gcr.io/cloudrun/hello"
}

variable "pubsub_topic_name" {
  description = "Name of the Pub/Sub topic receiving governance events."
  type        = string
  default     = "agentgate-governance-events"
}

variable "bq_dataset_id" {
  description = "BigQuery dataset holding governance event tables."
  type        = string
  default     = "agentgate"
}

variable "bq_location" {
  description = "BigQuery dataset location. Defaults to US multi-region; override to a single region (e.g. us-central1) if data residency matters."
  type        = string
  default     = "US"
}

variable "bq_table_id" {
  description = "BigQuery table that Pub/Sub writes governance events into."
  type        = string
  default     = "governance_events"
}

variable "artifact_repo_id" {
  description = "Artifact Registry Docker repo ID for AgentGate images."
  type        = string
  default     = "agentgate"
}

variable "secret_id_gemini_api_key" {
  description = "Secret Manager secret ID for the Gemini API key. The initial version is created out-of-band (`gcloud secrets versions add`)."
  type        = string
  default     = "gemini-api-key"
}

variable "github_repository" {
  description = "GitHub repo in 'owner/name' form, used as the WIF attribute condition so only this repo can impersonate the CI service account."
  type        = string
}

variable "allow_public_invoke" {
  description = "Whether allUsers can invoke the Cloud Run service. Useful for portfolio demo; flip to false once auth is added."
  type        = bool
  default     = true
}
