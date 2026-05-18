# AgentGate Runbook

Operational reference for the AgentGate service. Written so someone on-call at 3am can find the answer without paging the author.

## Quick reference

| Need to... | Section |
|---|---|
| Deploy a new version | [Deploy](#deploy) |
| Roll back a bad deploy | [Rollback](#rollback) |
| Debug "Blocked: PII Detected" false positive | [PII false positive](#pii-false-positive) |
| Query governance events | [Inspect events in BigQuery](#inspect-events-in-bigquery) |
| Rotate the Gemini API key | [Rotate Gemini API key](#rotate-gemini-api-key) |
| Drain a Pub/Sub backlog | [Drain Pub/Sub backlog](#drain-pubsub-backlog) |
| Diagnose 5xx spikes | [5xx triage](#5xx-triage) |
| Tail logs | [Tail logs](#tail-logs) |

---

## Deploy

Standard path: merge to `main`. The `deploy` GitHub workflow builds the image, pushes to Artifact Registry tagged with the commit SHA, deploys a new Cloud Run revision, and smoke-tests `/healthz`.

If you need to deploy a specific commit out-of-band:

```bash
gcloud run deploy agentgate \
  --image=us-central1-docker.pkg.dev/agent-gate/agentgate/agentgate:<sha> \
  --region=us-central1
```

The image for any past commit is at `agentgate:<full-git-sha>` in Artifact Registry as long as it was built by CI.

## Rollback

Cloud Run keeps every revision. To roll back:

```bash
# List the last 5 revisions
gcloud run revisions list --service=agentgate --region=us-central1 --limit=5

# Route 100% of traffic to a specific revision
gcloud run services update-traffic agentgate \
  --region=us-central1 \
  --to-revisions=<revision-name>=100
```

Rollback is ~10 seconds. If the bad deploy is still building, cancel the GitHub Actions run first or it'll race with you.

## PII false positive

Symptom: legitimate response is returning `403 Blocked: PII Detected` (non-streaming) or `data: [BLOCKED: PII DETECTED]` (streaming).

**Step 1**: confirm what fired. Check the request ID in your logs and pull the matching governance event:

```bash
gcloud logging read \
  'resource.type="cloud_run_revision"
   AND resource.labels.service_name="agentgate"
   AND jsonPayload.request_id="<request-id>"' \
  --limit=20 --format=json | jq '.[].jsonPayload'
```

You're looking for `policy:"pii"` events. The `reason` field tells you which regex matched (`email_detected` or `phone_detected`).

**Step 2**: reproduce locally. The PII regexes live in `internal/policy/pii.go`. Paste the offending text into a test case in `internal/policy/pii_test.go` and run `go test ./internal/policy/...`.

**Step 3**: fix. Either tighten the regex (preferred — false positives mean the pattern is too loose) or, if the regex is correct and the input is genuinely PII-shaped, accept the block.

Do NOT silently disable the policy. Add a regression test for any change.

## Inspect events in BigQuery

Every governance decision lands in `agent-gate.agentgate.governance_events`. Common queries:

```sql
-- Recent blocks
SELECT timestamp, request_id, model, reason, latency_ms
FROM `agent-gate.agentgate.governance_events`
WHERE decision = 'abort'
ORDER BY timestamp DESC
LIMIT 50;

-- Per-day decision mix
SELECT DATE(timestamp) AS day, decision, COUNT(*) AS n
FROM `agent-gate.agentgate.governance_events`
WHERE timestamp >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 7 DAY)
GROUP BY day, decision
ORDER BY day DESC, decision;

-- p95 latency by streaming flag
SELECT streaming,
       APPROX_QUANTILES(latency_ms, 100)[OFFSET(95)] AS p95_ms
FROM `agent-gate.agentgate.governance_events`
WHERE timestamp >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 1 HOUR)
GROUP BY streaming;
```

If a query returns no rows but you know events should be flowing, see [Drain Pub/Sub backlog](#drain-pubsub-backlog).

## Rotate Gemini API key

1. Generate a new key in [Google AI Studio](https://aistudio.google.com/apikey).
2. Add it as a new Secret Manager version:
   ```bash
   echo -n "$NEW_KEY" | gcloud secrets versions add gemini-api-key --data-file=-
   ```
3. Cloud Run's secret reference uses `version: latest`, so the next revision picks up the new key. Force a revision restart:
   ```bash
   gcloud run services update agentgate --region=us-central1 --clear-env-vars NOOP_TRIGGER || true
   gcloud run services update agentgate --region=us-central1 --set-env-vars NOOP_TRIGGER=$(date +%s)
   ```
   (Cloud Run only restarts when something in the spec changes; the timestamp env var is a no-op trigger.)
4. Disable the old key version once you've verified the new one works:
   ```bash
   gcloud secrets versions disable <old-version-number> --secret=gemini-api-key
   ```

## Drain Pub/Sub backlog

If governance events stop showing up in BigQuery, the subscription may be backed up.

```bash
# Check backlog depth
gcloud pubsub subscriptions describe agentgate-governance-events-to-bq \
  --format="value(messageRetentionDuration, ackDeadlineSeconds)"

# Check undelivered count
gcloud monitoring metrics list --filter="pubsub.googleapis.com/subscription/num_undelivered_messages"
```

Common causes:

- **BigQuery quota exceeded** — Pub/Sub BQ subscriptions write per-row; check the BigQuery streaming insert quota in the console.
- **Schema mismatch** — the subscription writes the message payload as-is. If we changed `GovernanceEvent` without updating `governance_events_schema.json`, writes will fail. Check Cloud Logging for `pubsub_subscriber` errors.
- **IAM** — the Pub/Sub service agent must have `roles/bigquery.dataEditor`. This is set in `infra/terraform/pubsub.tf` but if someone removed it manually, re-apply Terraform.

## 5xx triage

```bash
# Recent 5xx counts by status
gcloud logging read \
  'resource.type="cloud_run_revision"
   AND resource.labels.service_name="agentgate"
   AND httpRequest.status >= 500' \
  --limit=50 --format='value(timestamp, httpRequest.status, jsonPayload.err)'
```

Most likely causes:

| Sample log | Cause | Fix |
|---|---|---|
| `gemini generate failed` | upstream Gemini error | check AI Studio status; check key validity; check quota |
| `pubsub publish failed` | Pub/Sub auth or quota | check runtime SA has `pubsub.publisher`; check Pub/Sub quota |
| connection refused on healthz | container failed to start | check Cloud Run revision logs at startup |

## Tail logs

```bash
gcloud beta run services logs tail agentgate --region=us-central1
```

For structured queries against historical logs:

```bash
gcloud logging read \
  'resource.type="cloud_run_revision" AND resource.labels.service_name="agentgate"' \
  --limit=100 --format=json | jq '.[] | .jsonPayload // .textPayload'
```

Every log line includes `request_id` when emitted from within a request scope — filter on it to follow a single request end-to-end.

## Useful URLs

- Cloud Run service: https://console.cloud.google.com/run/detail/us-central1/agentgate
- Artifact Registry: https://console.cloud.google.com/artifacts/docker/agent-gate/us-central1/agentgate
- BigQuery dataset: https://console.cloud.google.com/bigquery?project=agent-gate&d=agentgate
- Pub/Sub topic: https://console.cloud.google.com/cloudpubsub/topic/detail/agentgate-governance-events?project=agent-gate
- Secret Manager: https://console.cloud.google.com/security/secret-manager?project=agent-gate
