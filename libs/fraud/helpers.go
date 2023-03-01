package fraud

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

func PubsubTopicID(fraudType, networkID string) string {
	return fmt.Sprintf("/%s/fraud-sub/%s/v0.0.1", networkID, fraudType)
}

func protocolID(networkID string) protocol.ID {
	return protocol.ID(fmt.Sprintf("/%s/fraud/v0.0.1", networkID))
}

func join(p *pubsub.PubSub, proofType ProofType, networkID string,
	validate func(context.Context, ProofType, peer.ID, *pubsub.Message) pubsub.ValidationResult) (*pubsub.Topic, error) {
	topic := PubsubTopicID(proofType.String(), networkID)
	log.Infow("joining topic", "id", topic)
	t, err := p.Join(topic)
	if err != nil {
		return nil, err
	}
	err = p.RegisterTopicValidator(
		topic,
		func(ctx context.Context, from peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			return validate(ctx, proofType, from, msg)
		},
	)
	return t, err
}
