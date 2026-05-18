package events

type MultiEmitter struct {
	emitters []Emitter
}

func NewMultiEmitter(emitters ...Emitter) *MultiEmitter {
	return &MultiEmitter{emitters: emitters}
}

func (m *MultiEmitter) Emit(event GovernanceEvent) {
	for _, e := range m.emitters {
		e.Emit(event)
	}
}
