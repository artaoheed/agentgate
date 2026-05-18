# AgentGate — DevOps Portfolio Implementation Plan (Tier 1)

This plan covers the **table-stakes DevOps work** needed before this repo reads as "production-ready service" to a hiring manager. Five phases, each ~1 evening session. Phases are ordered by dependency, not importance.

The deliberate goal: a hiring manager can clone this repo, read the README, and within 10 minutes understand the deploy story, the observability story, and the security posture — without running the code.

---

## Phase 0 — Prerequisites & Decisions

Lock these down before any code lands. Each decision affects Terraform variables and CI workflow inputs.

| Decision | Options | Recommendation |
|---|---|---|
| GCP project ID | new dedicated project vs. existing | **New dedicated project** (e.g. `agentgate-prod-<unique>`) — clean IAM, easy teardown |
| Region | `us-central1`, `us-east1`, `europe-west1`, etc. | `us-central1` (cheapest, all services GA) |
| Compute target | Cloud Run vs. GKE | **Cloud Run** — fewer moving parts, autoscale-to-zero, perfect signal for a stateless HTTP gateway. GKE only if the role specifically asks for k8s. |
| CI identity | Service-account JSON key vs. Workload Identity Federation (OIDC) | **WIF** — modern, no static keys in GitHub secrets |
| Domain | none (Cloud Run default URL) vs. custom domain | none for now; map custom domain later if desired |
| Image registry | Artifact Registry (recommended) vs. GCR (legacy) | Artifact Registry |

**Action required from you before Phase 2 starts:**
- Create the GCP project, enable billing, note the project ID.
- Decide on region (default `us-central1` if unsure).
- Confirm Cloud Run (default) vs. GKE.

---

## Phase 1 — Observability quick-wins (no GCP needed)

**Goal:** the app exposes the three signals every prod service should have.

**Tasks:**
1. Add `/readyz` endpoint — returns 503 until startup checks pass (Pub/Sub client init succeeded OR was intentionally disabled), then 200.
2. Add `/metrics` endpoint using `github.com/prometheus/client_golang`. Emit:
   - `agentgate_requests_total{path,method,status}` (counter)
   - `agentgate_request_duration_seconds{path}` (histogram)
   - `agentgate_governance_decisions_total{policy,decision,reason}` (counter — emitted from the existing emitter layer via a new `MetricsEmitter`)
   - `agentgate_gemini_requests_total{model,outcome}` and `agentgate_gemini_duration_seconds`
3. Replace `log.Printf` with `log/slog` (stdlib, no new dep). JSON handler in prod, text handler when `ENV=dev`.
4. Add `request_id` to every log line via slog attributes.

**Files touched:** `cmd/server/main.go`, new `internal/events/metrics_emitter.go`, new `internal/obs/logger.go`. New tests: `metrics_emitter_test.go`.

**Acceptance:** `curl /metrics` returns Prometheus exposition format; every log line is one JSON object; `/readyz` returns 503 until first successful Pub/Sub publish (or after a 5s grace if Pub/Sub disabled).

**Effort:** ~3 hours.

**Hiring signal:** "candidate knows the observability triad and wires it up by default."

---

## Phase 2 — Terraform foundation

**Goal:** every piece of GCP infrastructure for this project is described in code, applied via one `terraform apply`.

**Structure:**
```
infra/
  terraform/
    main.tf          # provider, project locals
    apis.tf          # google_project_service for each enabled API
    pubsub.tf        # topic + bigquery subscription
    bigquery.tf      # dataset + table from governance_events_schema.json
    cloud_run.tf     # service, scaling, env, secrets
    iam.tf           # service accounts + role bindings
    secrets.tf       # secret_manager_secret for GEMINI_API_KEY
    artifact_registry.tf
    cicd.tf          # workload_identity_pool + provider, GitHub binding
    variables.tf
    outputs.tf
    terraform.tfvars.example
  README.md          # "how to bootstrap" runbook
```

**Resources provisioned:**
- 6 enabled APIs: `run`, `pubsub`, `bigquery`, `secretmanager`, `artifactregistry`, `iamcredentials`
- `google_pubsub_topic.governance_events`
- `google_bigquery_dataset.agentgate` + `google_bigquery_table.governance_events` (schema from existing JSON)
- `google_pubsub_subscription.bq_sink` (type: BigQuery, writes events into the table)
- `google_artifact_registry_repository.containers`
- `google_secret_manager_secret.gemini_api_key` (+ initial version applied out-of-band via `gcloud`)
- `google_service_account.runtime` (for Cloud Run) and `google_service_account.cicd` (for GitHub Actions)
- IAM bindings: runtime SA gets `pubsub.publisher`, `secretmanager.secretAccessor`; cicd SA gets `run.admin`, `artifactregistry.writer`, `iam.serviceAccountUser`
- `google_iam_workload_identity_pool` + provider for GitHub OIDC (Phase 4 will use this)
- `google_cloud_run_v2_service.agentgate` — wired to runtime SA, with Pub/Sub topic / project / region as env vars, and `GEMINI_API_KEY` mounted from Secret Manager

**State backend:** `gcs` backend in a dedicated state bucket — also Terraformed via a tiny bootstrap `infra/bootstrap/` module that creates only the state bucket (chicken-and-egg).

**Acceptance:** `cd infra/terraform && terraform apply` provisions everything from scratch into a fresh project; `terraform destroy` removes it cleanly.

**Effort:** ~6 hours (this is the biggest phase).

**Hiring signal:** "candidate provisions infra-as-code, including the CI identity bootstrap."

---

## Phase 3 — Wire app to managed secrets + identity

**Goal:** drop static credentials from local + container environments; rely on ADC + Secret Manager.

**Tasks:**
1. `cmd/server/main.go`: if `GEMINI_API_KEY` is not in env, look up `GEMINI_API_KEY_SECRET` (full Secret Manager resource name) and fetch the version. New `internal/secrets/manager.go` does the lookup using `cloud.google.com/go/secretmanager`.
2. Cloud Run service is already configured (in Phase 2 Terraform) to inject `GEMINI_API_KEY` from Secret Manager — this code path is a fallback for non-Cloud-Run hosts.
3. Document local dev: developers either `export GEMINI_API_KEY=...` from AI Studio OR `gcloud secrets versions access latest --secret=gemini-api-key | export GEMINI_API_KEY=$(cat)`.
4. Delete `grafana-bq-key.json` from local disk and rotate the underlying key in GCP console (it's not in git, but it's a static key that should be retired).

**Files touched:** `cmd/server/main.go`, new `internal/secrets/`, README dev-setup section.

**Acceptance:** running on Cloud Run, the container has no static credentials in env or filesystem; locally, dev can either set env directly or pull from Secret Manager.

**Effort:** ~2 hours.

**Hiring signal:** "candidate doesn't ship static service-account keys."

---

## Phase 4 — GitHub Actions CI/CD

**Goal:** a push to `main` lints, tests, builds an image, pushes to Artifact Registry, and deploys to Cloud Run — all without static credentials in GitHub secrets.

**Workflows:**
1. `.github/workflows/ci.yml` — runs on every PR + push:
   - `go vet`, `staticcheck`, `gofmt -l` (fail on diff)
   - `go test ./... -race -cover` with coverage upload
   - `trivy` scan on a local image build
2. `.github/workflows/deploy.yml` — runs on push to `main`:
   - WIF auth via `google-github-actions/auth@v2` (uses the pool/provider from Phase 2)
   - Build + push image to Artifact Registry, tagged with git SHA
   - `gcloud run deploy` with `--image` and `--region` from repo variables
   - Smoke test: `curl $(gcloud run services describe ... --format='value(status.url)')/livez`
3. `.github/workflows/terraform.yml` — runs on PR touching `infra/**`:
   - `terraform fmt -check`, `terraform validate`, `terraform plan` posted as PR comment
   - On push to `main`: `terraform apply` with manual approval gate (GitHub environment)

**Branch protection:** require CI green + 1 approval (you'll self-approve as solo dev, that's fine for a portfolio).

**Acceptance:** PR shows green checks; merge to `main` produces a Cloud Run revision visible in console within ~5 minutes; smoke test passes.

**Effort:** ~3 hours.

**Hiring signal:** "candidate ships through a proper pipeline, no manual deploys, no JSON keys."

---

## Phase 5 — README rewrite + architecture diagram

**Goal:** the top of the README sells the project as production-ready in 30 seconds.

**New sections (in order):**
1. **Hero**: one-line pitch + CI badge + deploy badge + image-scan badge
2. **Live demo** (optional): link to deployed Cloud Run URL with curl examples
3. **Architecture diagram**: SVG showing Client → Cloud Run (AgentGate) → Gemini, with side-arrows to Pub/Sub → BigQuery for governance events. Generated via `d2` or `mermaid` (rendered inline on GitHub).
4. **Quick start (local)** — already present, keep
5. **Deploy** — `cd infra/terraform && terraform apply` then push to main, that's it
6. **Observability** — list of metrics, sample PromQL, link to a sample Grafana dashboard JSON in `docs/dashboards/`
7. **Runbook** — link to `docs/RUNBOOK.md` (separate file: how to deploy, rollback, debug PII false-positive, drain Pub/Sub backlog)
8. **Security posture** — distroless, non-root, WIF, Secret Manager, least-privilege IAM bindings table
9. **Cost** — back-of-envelope per 1M requests (Gemini Flash + Cloud Run + Pub/Sub + BigQuery)
10. **Roadmap** — Tier 2/3 items from the assessment

**Files touched:** `README.md` (full rewrite), new `docs/architecture.svg` (or `.mmd`), new `docs/RUNBOOK.md`, new `docs/dashboards/agentgate.json`.

**Effort:** ~3 hours.

**Hiring signal:** "candidate communicates clearly and treats docs as a deliverable."

---

## Out of scope for Tier 1 (call out for honesty, defer to Tier 2/3)

- Distributed tracing (OpenTelemetry — deps present but not wired)
- SLO doc + alert policies (Cloud Monitoring alerting)
- Load test results in README (k6 script + a chart)
- Auth on the `/v1/chat/completions` endpoint
- Rate limiting
- Canary deploys via Cloud Run revision tags
- Helm chart (only if pivoting to GKE)
- Pre-commit hooks / Makefile

These are Tier 2 — once Tier 1 is shipped, pick 2–3 based on the role's emphasis.

---

## Total effort estimate

| Phase | Effort | Cumulative |
|---|---|---|
| 0 — decisions | 30 min | 0:30 |
| 1 — observability | 3h | 3:30 |
| 2 — Terraform | 6h | 9:30 |
| 3 — secrets | 2h | 11:30 |
| 4 — CI/CD | 3h | 14:30 |
| 5 — README | 3h | 17:30 |

Realistically: **3–4 evening sessions** if focused, **1–2 weekends** if interrupted. Don't try to do it in one sitting.

---

## Suggested execution order

Phase 0 → 1 → 2 → 3 → 4 → 5.

Phase 1 first because it's independent and improves debugging during later phases. Phase 2 must precede Phase 4 (CI deploys via Terraform-provisioned identity). Phase 3 sits between because Cloud Run config (in Phase 2) needs the Secret Manager wiring (Phase 3) — but they can be merged into one session if pacing requires.
