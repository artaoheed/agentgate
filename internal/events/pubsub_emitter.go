package events

import (
	"context"
	"encoding/json"
	"log"

	"cloud.google.com/go/pubsub"
)

type PubSubEmitter struct {
	ctx context.Context
	client *pubsub.Client
	topic  *pubsub.Topic
}



func NewPubSubEmitter(ctx context.Context, projectID, topicID string) (*PubSubEmitter, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	topic := client.Topic(topicID)

	return &PubSubEmitter{
		ctx:   ctx,
		client: client,
		topic: topic,
	}, nil
}


func (e *PubSubEmitter) Emit(event GovernanceEvent) {
	b, err := json.Marshal(event)
	if err != nil {
		log.Printf("pubsub marshal failed: %v", err)
		return
	}

	log.Printf("DEBUG: publishing governance event to Pub/Sub: %+v", event)

	res := e.topic.Publish(e.ctx, &pubsub.Message{
		Data: b,
		Attributes: map[string]string{
			"policy":   event.Policy,
			"decision": string(event.Decision),
		},
	})

	if _, err := res.Get(e.ctx); err != nil {
		log.Printf("pubsub publish failed: %v", err)
	} else {
		log.Printf("DEBUG: governance event published successfully")
	}
}
