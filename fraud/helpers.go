package fraud

import (
	"context"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

const fraudSubSuffix = "-sub"

func getSubTopic(p ProofType) string {
	return p.String() + fraudSubSuffix
}

func getProofTypeFromTopic(topic string) ProofType {
	str := strings.TrimSuffix(topic, fraudSubSuffix)
	return toProof(str)
}

func toProof(proof string) ProofType {
	switch proof {
	case "badencoding":
		return BadEncoding
	default:
		panic(fmt.Sprintf("fraud: invalid proof type: %s", proof))
	}
}

func join(p *pubsub.PubSub, proofType ProofType,
	validate func(context.Context, ProofType, *pubsub.Message) pubsub.ValidationResult) (*pubsub.Topic, error) {
	t, err := p.Join(getSubTopic(proofType))
	if err != nil {
		return nil, err
	}
	err = p.RegisterTopicValidator(
		getSubTopic(proofType),
		func(ctx context.Context, _ peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			return validate(ctx, proofType, msg)
		},
	)
	return t, err
}
