package events

import (
	"encoding/json"
	"log"
)

type Emitter interface {
	Emit(event GovernanceEvent)
}

type LogEmitter struct{}

func NewLogEmitter() *LogEmitter {
	return &LogEmitter{}
}

func (e *LogEmitter) Emit(event GovernanceEvent) {
	go func() {
		b, err := json.Marshal(event)
		if err != nil {
			log.Printf("event marshal failed: %v", err)
			return
		}
		log.Printf("GOVERNANCE EVENT: %s", string(b))
	}()
}
