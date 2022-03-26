package fraud

import (
	"context"
	"errors"

	"github.com/celestiaorg/celestia-node/service/header"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tendermint/tendermint/libs/sync"
)

var log = logging.Logger("fraud")

type proofUnmarshaller func([]byte) (Proof, error)

type headerFetcher func(ctx context.Context, height uint64) (*header.ExtendedHeader, error)

// FraudSub implements Subscriber and Broadcaster.
type FraudSub struct {
	pubsub        *pubsub.PubSub
	topics        map[ProofType]*pubsub.Topic
	unmarshallers map[ProofType]proofUnmarshaller

	mu *sync.Mutex
}

func NewFraudSub(p *pubsub.PubSub) *FraudSub {
	return &FraudSub{pubsub: p, mu: &sync.Mutex{}}
}

func (f *FraudSub) Subscribe(proofType ProofType) (Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.topics[proofType]; ok {
		return nil, errors.New("alredy subscribed to current topic")
	}
	if _, ok := f.unmarshallers[proofType]; !ok {
		return nil, errors.New("unmarshaller is not registered")
	}

	topic, err := getSubTopic(proofType)
	if err != nil {
		return nil, err
	}

	err = f.subcribeTo(proofType, topic)
	if err != nil {
		return nil, err
	}

	return newSubscription(f.topics[proofType], f.unmarshallers[proofType])
}

func getSubTopic(p ProofType) (string, error) {
	switch p {
	case BadEncoding:
		return "befp-sub", nil
	default:
		return "", errors.New("invalid proof type")
	}
}

func (f *FraudSub) subcribeTo(p ProofType, t string) error {
	topic, err := f.pubsub.Join(t)
	if err != nil {
		return err
	}
	f.topics[p] = topic

	return nil
}

func (f *FraudSub) RegisterUnmarshaller(proofType ProofType, u proofUnmarshaller) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.unmarshallers[proofType]; ok {
		return errors.New("unmarshaller is registered")
	}
	f.unmarshallers[proofType] = u

	return nil
}

func (f *FraudSub) UnregisterUnmarshaller(proofType ProofType) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.unmarshallers[proofType]; !ok {
		return errors.New("unmarshaller is not registered")
	}
	delete(f.unmarshallers, proofType)

	return nil

}

func (f *FraudSub) Broadcast(ctx context.Context, p Proof) error {
	bin, err := p.MarshalBinary()
	if err != nil {
		return err
	}
	if topic, ok := f.topics[p.Type()]; ok {
		topic.Publish(ctx, bin)
	}

	return errors.New("topic is not found")
}

func (f *FraudSub) AddValidator(topic string, proofType ProofType, fetcher headerFetcher) error {
	val := func(ctx context.Context, p peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		f.mu.Lock()
		defer f.mu.Unlock()
		unmarshaller, ok := f.unmarshallers[proofType]
		if !ok {
			log.Error("unmarshaller is not found")
			return pubsub.ValidationReject
		}
		proof, err := unmarshaller(msg.Data)
		if err != nil {
			log.Errorw("unmarshalling header",
				"from", p.ShortString(),
				"err", err)
			return pubsub.ValidationReject
		}

		extHeader, err := fetcher(ctx, proof.Height())
		if err != nil {
			log.Errorw("failed to fetch block: ",
				"err", err)
			return pubsub.ValidationReject
		}

		err = proof.Validate(extHeader)
		if err != nil {
			log.Errorw("validation err: ",
				"err", err)
			return pubsub.ValidationReject
		}

		return pubsub.ValidationAccept
	}

	return f.pubsub.RegisterTopicValidator(topic, val)
}
