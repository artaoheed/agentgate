package events

type GovernanceEvent struct {
	Timestamp string `json:"timestamp"`   // RFC3339 string
	RequestID string `json:"request_id"`
	Model     string `json:"model"`
	Policy    string `json:"policy"`
	Decision  string `json:"decision"`    // allow | redact | abort
	Reason    string `json:"reason"`      // ALWAYS string ("" if none)
	Streaming bool   `json:"streaming"`
	LatencyMs int64  `json:"latency_ms"`
}
