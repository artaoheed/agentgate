# AgentGate Infrastructure

Terraform that provisions all GCP resources AgentGate needs to run in production. Two modules:

- `bootstrap/` — creates the GCS bucket that holds remote state for the main module. Run once per project.
- `terraform/` — main stack: APIs, Pub/Sub topic, BigQuery dataset+table+subscription, Artifact Registry, Secret Manager, IAM, Cloud Run service, Workload Identity Federation for GitHub Actions.

## Prerequisites

- `gcloud` CLI authenticated as a user with `Owner` (or sufficient IAM) on the target project.
- `terraform` >= 1.6.
- Application Default Credentials configured locally:
  ```bash
  gcloud auth application-default login
  ```
- A GCP project with billing enabled.

## First-time setup

### 1. Bootstrap the state bucket

```bash
cd bootstrap
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars   # set project_id
terraform init
terraform apply
```

This creates `${project_id}-agentgate-tfstate` in your chosen region. Note the bucket name from the output.

### 2. Apply the main stack

```bash
cd ../terraform
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars   # set project_id and github_repository
terraform init -backend-config="bucket=${project_id}-agentgate-tfstate"
terraform plan
terraform apply
```

This typically takes 3–5 minutes (Cloud Run service provisioning is the longest step).

### 3. Add the Gemini API key to Secret Manager

Terraform deliberately doesn't own the secret material (keeps it out of state):

```bash
echo -n "$GEMINI_API_KEY" | gcloud secrets versions add gemini-api-key --data-file=-
```

### 4. Verify

```bash
terraform output cloud_run_url
curl "$(terraform output -raw cloud_run_url)/livez"
```

You should see `ok`. (Note: `/readyz` will return 503 until the deployed container starts — the placeholder `gcr.io/cloudrun/hello` image doesn't implement it. After CI/CD pushes a real AgentGate image, `/readyz` will return 200.)

Liveness is `/livez` rather than `/healthz` because Google's edge intercepts `/healthz` on the default `*.run.app` domain and the request never reaches the container.

## Outputs to feed into GitHub Actions

Run `terraform output` after apply and stash these as repo variables / secrets:

| Output | Where it goes |
|---|---|
| `cicd_service_account` | GitHub Actions repo variable `GCP_CICD_SA` |
| `wif_provider` | GitHub Actions repo variable `GCP_WIF_PROVIDER` |
| `artifact_registry_repo` | GitHub Actions repo variable `ARTIFACT_REPO` |
| `cloud_run_url` | Display in README badge / runbook |

Phase 4 (CI/CD workflow) consumes these.

## Tearing down

```bash
cd terraform && terraform destroy
cd ../bootstrap && terraform destroy   # only if you also want the state bucket gone
```

`google_bigquery_table` has `deletion_protection = false` so destroy works without manual intervention; flip that to `true` once you have data you care about.

## What this module deliberately does NOT do

- **Manage the secret value** — only the secret resource. Add versions via `gcloud`.
- **Manage the Cloud Run image** — `lifecycle.ignore_changes` lets CI/CD update the image without Terraform fighting it.
- **Create the GitHub repo or branch protection** — out of scope; configure in GitHub.
- **Configure custom domain / SSL** — Cloud Run default URL is used; map a domain manually if needed.
