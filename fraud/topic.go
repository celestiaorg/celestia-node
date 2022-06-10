package fraud

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// topic holds pubsub topic and unmarshaler of corresponded Fraud Proof
type topic struct {
	topic *pubsub.Topic
	codec ProofUnmarshaler
}

// publish allows to publish Fraud Proofs to the network
func (t *topic) publish(ctx context.Context, data []byte) error {
	return t.topic.Publish(ctx, data)
}

// close removes unmarshaler and closes pubsub topic
func (t *topic) close() error {
	t.codec = nil
	return t.topic.Close()
}

func createTopic(
	p *pubsub.PubSub,
	proofType ProofType,
	codec ProofUnmarshaler,
	val func(context.Context, ProofType, *pubsub.Message) pubsub.ValidationResult) (*topic, error) {
	t, err := p.Join(getSubTopic(proofType))
	if err != nil {
		return nil, err
	}
	if err = registerValidator(p, proofType, val); err != nil {
		return nil, err
	}
	log.Debugf("successfully subscribed to topic: %s", getSubTopic(proofType))
	return &topic{topic: t, codec: codec}, nil
}

// registerValidator adds an internal validation to topic inside libp2p for provided ProofType.
func registerValidator(
	p *pubsub.PubSub,
	proofType ProofType,
	val func(context.Context, ProofType, *pubsub.Message) pubsub.ValidationResult,
) error {
	return p.RegisterTopicValidator(
		getSubTopic(proofType),
		func(ctx context.Context, _ peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			return val(ctx, proofType, msg)
		},
		// make validation synchronous.
		pubsub.WithValidatorInline(true),
	)
}
