package events

import (
	"encoding/json"
	"log/slog"
)

type Emitter interface {
	Emit(event GovernanceEvent)
}

// LogEmitter writes governance events to slog at info level. Marshals to
// JSON so downstream log scrapers (Cloud Logging, Loki, etc.) can index
// individual fields.
type LogEmitter struct {
	log *slog.Logger
}

func NewLogEmitter(log *slog.Logger) *LogEmitter {
	return &LogEmitter{log: log}
}

func (e *LogEmitter) Emit(event GovernanceEvent) {
	b, err := json.Marshal(event)
	if err != nil {
		e.log.Error("event marshal failed", "err", err)
		return
	}
	e.log.Info("governance_event", "event", json.RawMessage(b))
}
