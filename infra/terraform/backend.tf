# State backend. Bucket is provisioned by infra/bootstrap before the
# main module is applied. Bucket name is project-scoped:
#   <project_id>-agentgate-tfstate
# Override via `-backend-config=bucket=...` if you used a different
# name in bootstrap.
terraform {
  backend "gcs" {
    prefix = "agentgate/prod"
  }
}
