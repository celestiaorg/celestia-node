package header

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// PubSubTopic hardcodes the name of the ExtendedHeader
// gossipsub topic.
const PubSubTopic = "header-sub"

// PubsubManager manages the lifecycle and relationship of header Service
// with the "header-sub" gossipsub topic.
type PubsubManager struct {
	pubsub *pubsub.PubSub
	topic  *pubsub.Topic

	validator pubsub.ValidatorEx
}

// NewPubsubManager returns a PubsubManager that manages the header Service's
// relationship with the "header-sub" gossipsub topic.
func NewPubsubManager(ps *pubsub.PubSub, validator pubsub.ValidatorEx) *PubsubManager {
	return &PubsubManager{
		pubsub:    ps,
		validator: validator,
	}
}

// Start starts the pubsub manager, registering a topic validator for the "header-sub"
// topic and joining it.
func (pm *PubsubManager) Start(context.Context) error {
	err := pm.pubsub.RegisterTopicValidator(PubSubTopic, pm.validator)
	if err != nil {
		return err
	}

	pm.topic, err = pm.pubsub.Join(PubSubTopic)
	return err
}

// Stop closes the topic and unregisters its validator.
func (pm *PubsubManager) Stop(context.Context) error {
	err := pm.pubsub.UnregisterTopicValidator(PubSubTopic)
	if err != nil {
		return err
	}

	return pm.topic.Close()
}

// Subscribe returns a new subscription to the PubsubManager's
// topic.
func (pm *PubsubManager) Subscribe() (Subscription, error) {
	if pm.topic == nil {
		return nil, fmt.Errorf("header topic is not instantiated, service must be started before subscribing")
	}

	return newSubscription(pm.topic)
}
