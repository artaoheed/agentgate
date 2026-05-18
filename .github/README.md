# GitHub Actions for AgentGate

Three workflows live here:

| File | Trigger | What it does |
|---|---|---|
| `workflows/ci.yml` | every PR + push to main | gofmt, go vet, staticcheck, race-enabled tests with coverage, Trivy CRITICAL/HIGH scan of a freshly built image |
| `workflows/deploy.yml` | push to main, manual dispatch | WIF auth → build → push image tagged with git SHA to Artifact Registry → `gcloud run deploy` → smoke test `/livez` |
| `workflows/terraform.yml` | PRs that touch `infra/**` | `terraform fmt -check`, `terraform validate` against both modules |

## First-time setup

Phase 2's Terraform (`infra/terraform/`) provisions the Workload Identity Federation pool, the CI/CD service account, and the IAM bindings. After running `terraform apply`, copy these outputs into the GitHub repo as **Variables** (Settings → Secrets and variables → Actions → Variables):

| Variable | Value | Source |
|---|---|---|
| `GCP_PROJECT_ID` | `agent-gate` | tfvars |
| `GCP_REGION` | `us-central1` | tfvars |
| `GCP_WIF_PROVIDER` | `projects/<num>/locations/global/workloadIdentityPools/agentgate-github/providers/github` | `terraform output -raw wif_provider` |
| `GCP_CICD_SA` | `agentgate-cicd@agent-gate.iam.gserviceaccount.com` | `terraform output -raw cicd_service_account` |
| `ARTIFACT_REPO` | `us-central1-docker.pkg.dev/agent-gate/agentgate` | `terraform output -raw artifact_registry_repo` |
| `CLOUD_RUN_SERVICE` | `agentgate` | tfvars (`service_name`) |

**No secrets are required.** Authentication uses Workload Identity Federation — GitHub mints an OIDC token, GCP exchanges it for short-lived credentials. No static JSON key sits in GitHub.

## Adding the `production` environment

`deploy.yml` runs under the GitHub environment named `production`. Create it under Settings → Environments → New environment, then optionally add:

- **Required reviewers** — pause deploys for manual approval
- **Deployment branch rules** — only `main` can deploy

Even without protection rules, the environment groups deploy history and exposes scoped variables/secrets if you ever need them.

## Why no `terraform plan` in CI

Running `terraform plan` from CI needs read access to the GCS state bucket and at least `viewer` on every resource type managed by the module. Granting that to the deploy SA would expand its blast radius beyond what app deploys need. Cleaner options for later:

- Create a separate `agentgate-infra-ci` SA with `viewer` + state-bucket read, used only by the terraform workflow.
- Or use [Atlantis](https://www.runatlantis.io/) / [Spacelift](https://spacelift.io/) instead of GitHub Actions for plan-as-PR-comment.

Until then, `terraform apply` is run locally from the project root (`infra/terraform/`).
