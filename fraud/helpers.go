package fraud

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

const fraudSubSuffix = "-sub"

func getSubTopic(p ProofType) string {
	return p.String() + fraudSubSuffix
}

func join(p *pubsub.PubSub, proofType ProofType,
	validate func(context.Context, ProofType, peer.ID, *pubsub.Message) pubsub.ValidationResult) (*pubsub.Topic, error) {
	t, err := p.Join(getSubTopic(proofType))
	if err != nil {
		return nil, err
	}
	err = p.RegisterTopicValidator(
		getSubTopic(proofType),
		func(ctx context.Context, from peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			return validate(ctx, proofType, from, msg)
		},
	)
	return t, err
}
