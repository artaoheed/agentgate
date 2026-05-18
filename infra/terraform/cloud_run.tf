resource "google_cloud_run_v2_service" "agentgate" {
  name     = var.service_name
  location = var.region

  template {
    service_account = google_service_account.runtime.email

    scaling {
      min_instance_count = 0
      max_instance_count = 5
    }

    containers {
      image = var.image

      ports {
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
      }

      env {
        name  = "GOOGLE_CLOUD_PROJECT"
        value = var.project_id
      }

      env {
        name = "GEMINI_API_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.gemini_api_key.secret_id
            version = "latest"
          }
        }
      }

      # NOTE: liveness path is /livez, not the conventional /healthz —
      # Google's edge intercepts /healthz on the *.run.app domain and
      # the request never reaches the container.
      startup_probe {
        http_get {
          path = "/readyz"
        }
        initial_delay_seconds = 2
        period_seconds        = 5
        timeout_seconds       = 2
        failure_threshold     = 6
      }

      liveness_probe {
        http_get {
          path = "/livez"
        }
        period_seconds = 30
      }
    }
  }

  labels = local.labels

  lifecycle {
    # CI/CD updates the image on each deploy; Terraform shouldn't
    # revert it on the next `terraform apply`.
    ignore_changes = [
      template[0].containers[0].image,
    ]
  }

  depends_on = [
    google_secret_manager_secret_iam_member.runtime_secret_accessor,
    google_pubsub_topic_iam_member.runtime_publisher,
  ]
}

resource "google_cloud_run_v2_service_iam_member" "public_invoker" {
  count = var.allow_public_invoke ? 1 : 0

  name     = google_cloud_run_v2_service.agentgate.name
  location = google_cloud_run_v2_service.agentgate.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}
