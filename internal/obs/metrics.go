// Package obs holds observability primitives — metrics, logger setup,
// and (eventually) tracing wiring. Importing this package registers the
// metrics with the default Prometheus registry as a side effect.
package obs

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequests counts inbound HTTP requests by path/method/status.
	// path is the registered route (e.g. "/v1/chat/completions"), not
	// the raw URL — keeps cardinality bounded.
	HTTPRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentgate_requests_total",
			Help: "Total HTTP requests handled by the gateway.",
		},
		[]string{"path", "method", "status"},
	)

	// HTTPDuration measures wall-clock time per request.
	HTTPDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentgate_request_duration_seconds",
			Help:    "Inbound request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path"},
	)

	// GovernanceDecisions counts policy outcomes emitted by the
	// governance event pipeline.
	GovernanceDecisions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentgate_governance_decisions_total",
			Help: "Total governance decisions emitted, by policy/decision/reason.",
		},
		[]string{"policy", "decision", "reason"},
	)

	// GeminiRequests counts upstream Gemini calls by model and outcome.
	GeminiRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentgate_gemini_requests_total",
			Help: "Total upstream Gemini calls.",
		},
		[]string{"model", "mode", "outcome"},
	)

	// GeminiDuration measures upstream Gemini call latency.
	GeminiDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agentgate_gemini_duration_seconds",
			Help:    "Upstream Gemini call duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"model", "mode"},
	)
)
