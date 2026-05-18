package events

import "github.com/artaoheed/agentgate/internal/obs"

// MetricsEmitter bumps the governance_decisions counter on every event.
// Designed to be combined with other emitters via MultiEmitter so log /
// pubsub / metrics stay decoupled.
type MetricsEmitter struct{}

func NewMetricsEmitter() *MetricsEmitter {
	return &MetricsEmitter{}
}

func (m *MetricsEmitter) Emit(event GovernanceEvent) {
	reason := event.Reason
	if reason == "" {
		reason = "none"
	}
	obs.GovernanceDecisions.WithLabelValues(
		event.Policy,
		event.Decision,
		reason,
	).Inc()
}
