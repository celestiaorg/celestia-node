package fraud

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

func join(p *pubsub.PubSub, proofType ProofType, protocolSuffix string,
	validate func(context.Context, ProofType, peer.ID, *pubsub.Message) pubsub.ValidationResult) (*pubsub.Topic, error) {
	topic := pubSubTopicID(string(proofType), protocolSuffix)
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

func pubSubTopicID(fraudType, protocolSuffix string) string {
	return "/fraud-sub/" + fraudType + "/v0.0.1/" + protocolSuffix
}

func protocolID(protocolSuffix string) protocol.ID {
	return protocol.ID(fmt.Sprintf("/fraud/v0.0.1/%s", protocolSuffix))
}
