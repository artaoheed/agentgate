# AgentGate

**AgentGate** is a Gemini-first gateway in Go that sits between your application and the Gemini API, evaluating responses for PII before they reach users and emitting structured governance events for downstream analytics.

It's built for teams running Gemini in production who need safety and auditability without giving up streaming UX.

---

## Why Gemini + Google AI Studio

AgentGate is built **Gemini-first**, intentionally.

- Native support for **Gemini streaming APIs**
- Optimized for **Gemini Flash and Gemini Pro**
- Designed around **Google AI Studio workflows**
- Built to deploy on **GCP (Cloud Run, Pub/Sub, BigQuery)**

---

## Shipped Today

### 🔐 Response-side PII enforcement
- Regex-based detection for **email** (Abort) and **phone numbers** (Redact)
- Rolling 300-char window so matches that span chunk boundaries are caught mid-stream
- Throttled mid-stream checks (~every 50 chars) plus a guaranteed final check on stream close
- Matched spans are masked in the buffer so a single hit doesn't re-fire on the next evaluation

### ⚡ Streaming gateway
- OpenAI-compatible `/v1/chat/completions` endpoint (request + response shape)
- SSE streaming passthrough with per-chunk policy evaluation
- Client-disconnect handling via `<-ctx.Done()`

### 📊 Governance event emission
- Structured `GovernanceEvent` (request ID, model, policy, decision, reason, latency, streaming flag)
- Multi-emitter: stdout log + Google Cloud Pub/Sub, fans out per event
- BigQuery-compatible schema (`governance_events_schema.json`) for downstream analytics
- Graceful degrade to log-only if Pub/Sub init fails

### 🛠 Operability
- `/healthz` liveness endpoint
- Graceful shutdown on SIGINT/SIGTERM, flushes Pub/Sub publisher
- Distroless container image, Go 1.24

---

## Roadmap

These are described in the project pitch but **not yet implemented**.

- **Prompt-side validation** — today only model responses are evaluated; user prompts pass through unchecked.
- **Custom policy hooks (sync + async)** — the policy layer is currently a hardcoded PII evaluator. A pluggable `Hook` interface would let teams add their own evaluators (safety, cost, relevance) without forking.
- **Semantic caching** — embedding-based response cache (Redis / vector backend) to cut cost and latency on repeated queries.
- **Modular middleware chain** — the handler is currently a single inline function; a middleware architecture would make hooks, auth, rate limiting, and metrics composable.

Other gaps worth flagging: no auth, no rate limiting, no `/metrics` endpoint, no request body size limit, model name is hardcoded (`gemini-2.5-flash`).

---

## High-Level Architecture

```text
Client
  │
  ▼
AgentGate (Go)
  ├─ Policy Engine  (PII: email→abort, phone→redact)
  ├─ Streaming Controller  (rolling window + throttled checks)
  ├─ Async Event Emitter
  │        │
  │        ▼
  │   Pub/Sub
  │        │
  │        ▼
  │   BigQuery
  │
  ▼
Gemini API (via Google AI Studio)
```

---

## Quick Start

```bash
export GEMINI_API_KEY=...
export GOOGLE_CLOUD_PROJECT=your-project    # optional; defaults to "agent-gate"
go run ./cmd/server
```

Then:

```bash
curl -s localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"messages":[{"role":"user","content":"say hi"}]}'
```

Add `"stream": true` for SSE.
