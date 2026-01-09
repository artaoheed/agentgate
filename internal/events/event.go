package events

import "time"

type Decision string

const (
	DecisionAllow  Decision = "allow"
	DecisionRedact Decision = "redact"
	DecisionAbort  Decision = "abort"
)

type GovernanceEvent struct {
	Timestamp time.Time `json: "timestamp"`
	RequestID string    `json: "request_id"`
	Model     string    `json: "model"`
	Policy    string    `json: "policy"`
	Decision  Decision  `json: "decision"`
	Reason    string    `json: "reason,omitempty"`
	Streaming bool      `json: "streaming"`
	LatencyMs int64     `json: "latency_ms"`
}


