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

      # TCP startup probe (not HTTP /readyz) so the spec applies
      # cleanly against pre-existing images that may not yet implement
      # that endpoint. Once a build of the current code is deployed
      # via CI/CD, switch to an HTTP probe on /readyz, and re-add a
      # liveness_probe on /healthz (Cloud Run only supports HTTP/gRPC
      # for liveness, not TCP, so it's omitted entirely here).
      startup_probe {
        tcp_socket {
          port = 8080
        }
        initial_delay_seconds = 2
        period_seconds        = 5
        timeout_seconds       = 2
        failure_threshold     = 6
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
