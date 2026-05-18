resource "google_secret_manager_secret" "gemini_api_key" {
  secret_id = var.secret_id_gemini_api_key

  replication {
    auto {}
  }

  labels = local.labels

  depends_on = [google_project_service.apis]
}

# NOTE: the secret VERSION is added out-of-band via:
#   echo -n "$GEMINI_API_KEY" | gcloud secrets versions add gemini-api-key --data-file=-
# Terraform deliberately does not own the key material — keeps secrets
# out of state.
