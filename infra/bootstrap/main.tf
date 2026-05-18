terraform {
  required_version = ">= 1.6"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

variable "project_id" {
  description = "GCP project ID."
  type        = string
}

variable "region" {
  description = "GCP region for the state bucket."
  type        = string
  default     = "us-central1"
}

# State bucket for the main Terraform module. Versioning is on so
# corrupted-state recovery is possible.
resource "google_storage_bucket" "tfstate" {
  name          = "${var.project_id}-agentgate-tfstate"
  location      = var.region
  force_destroy = false

  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      num_newer_versions = 10
    }
    action {
      type = "Delete"
    }
  }
}

output "state_bucket" {
  value = google_storage_bucket.tfstate.name
}
