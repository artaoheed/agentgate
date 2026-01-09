package events

import (
	"context"
	"encoding/json"
	"log"

	"cloud.google.com/go/pubsub"
)

type PubSubEmitter struct {
	client *pubsub.Client
	topic  *pubsub.Topic
}

func NewPubSubEmitter(ctx context.Context, projectID, topicName string) (*PubSubEmitter, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	topic := client.Topic(topicName)

	return &PubSubEmitter{
		client: client,
		topic:  topic,
	}, nil
}

func (e *PubSubEmitter) Emit(event GovernanceEvent) {
	go func() {
		b, err := json.Marshal(event)
		if err != nil {
			log.Printf("pubsub marshal failed: %v", err)
			return
		}

		res := e.topic.Publish(context.Background(), &pubsub.Message{
			Data: b,
			Attributes: map[string]string{
				"policy":   event.Policy,
				"decision": string(event.Decision),
			},
		})

		if _, err := res.Get(context.Background()); err != nil {
			log.Printf("pubsub publish failed: %v", err)
		}
	}()
}
