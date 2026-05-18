package events

import (
	"context"
	"encoding/json"
	"log/slog"

	"cloud.google.com/go/pubsub"
)

type PubSubEmitter struct {
	ctx    context.Context
	client *pubsub.Client
	topic  *pubsub.Topic
	log    *slog.Logger
}

func NewPubSubEmitter(ctx context.Context, projectID, topicID string, log *slog.Logger) (*PubSubEmitter, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return &PubSubEmitter{
		ctx:    ctx,
		client: client,
		topic:  client.Topic(topicID),
		log:    log,
	}, nil
}

// Close stops the topic publisher (flushing any pending messages) and
// closes the underlying client. Safe to call once at shutdown.
func (e *PubSubEmitter) Close() error {
	e.topic.Stop()
	return e.client.Close()
}

func (e *PubSubEmitter) Emit(event GovernanceEvent) {
	b, err := json.Marshal(event)
	if err != nil {
		e.log.Error("pubsub marshal failed", "err", err)
		return
	}

	res := e.topic.Publish(e.ctx, &pubsub.Message{
		Data: b,
		Attributes: map[string]string{
			"policy":   event.Policy,
			"decision": event.Decision,
		},
	})

	if _, err := res.Get(e.ctx); err != nil {
		e.log.Error("pubsub publish failed", "err", err, "request_id", event.RequestID)
	}
}
