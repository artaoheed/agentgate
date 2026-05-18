package events

import (
	"testing"

	"github.com/artaoheed/agentgate/internal/obs"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsEmitter_BumpsCounter(t *testing.T) {
	em := NewMetricsEmitter()

	before := testutil.ToFloat64(obs.GovernanceDecisions.WithLabelValues("pii", "abort", "email_detected"))
	em.Emit(GovernanceEvent{
		Policy:   "pii",
		Decision: "abort",
		Reason:   "email_detected",
	})
	after := testutil.ToFloat64(obs.GovernanceDecisions.WithLabelValues("pii", "abort", "email_detected"))

	if after-before != 1 {
		t.Errorf("counter delta = %v, want 1", after-before)
	}
}

func TestMetricsEmitter_EmptyReasonNormalized(t *testing.T) {
	em := NewMetricsEmitter()

	before := testutil.ToFloat64(obs.GovernanceDecisions.WithLabelValues("none", "allow", "none"))
	em.Emit(GovernanceEvent{
		Policy:   "none",
		Decision: "allow",
		Reason:   "",
	})
	after := testutil.ToFloat64(obs.GovernanceDecisions.WithLabelValues("none", "allow", "none"))

	if after-before != 1 {
		t.Errorf("empty-reason event should bump 'none' label; delta = %v", after-before)
	}
}
