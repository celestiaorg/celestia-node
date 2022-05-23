package fraud

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/celestiaorg/celestia-node/header"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

var log = logging.Logger("fraud")

// proofUnmarshaler aliases a function that parses data to `Proof`
type proofUnmarshaler func([]byte) (Proof, error)

// headerFetcher aliases a function that is used to fetch ExtendedHeader from store
type headerFetcher func(context.Context, uint64) (*header.ExtendedHeader, error)

// Validator aliases a function that validate pubsub incoming messages
type Validator func(ctx context.Context, proofType ProofType, data []byte) pubsub.ValidationResult

// service is propagating and validating Fraud Proofs
// service implements Service interface.
type service struct {
	pubsub       *pubsub.PubSub
	topics       map[ProofType]*pubsub.Topic
	unmarshalers map[ProofType]proofUnmarshaler
	getter       headerFetcher
	mu           *sync.Mutex
}

func NewService(p *pubsub.PubSub, hstore header.Store) Service {
	return &service{
		pubsub: p, topics: make(map[ProofType]*pubsub.Topic),
		unmarshalers: make(map[ProofType]proofUnmarshaler),
		getter:       hstore.GetByHeight,
		mu:           &sync.Mutex{},
	}
}

func (f *service) Start(context.Context) error {
	log.Info("fraud service is starting...")

	// Add validator and default Unmarshaler for BEFP
	err := f.AddValidator(BadEncoding, f.processIncoming)
	if err != nil {
		return err
	}
	err = f.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP)
	if err != nil {
		return err
	}
	return nil
}

func (f *service) Stop(context.Context) error {
	log.Info("fraud service is stopping...")
	return nil
}

func (f *service) Subscribe(proofType ProofType) (Subscription, error) {
	// TODO: @vgonkivs check if fraud proof is in fraud store, then return with error
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.unmarshalers[proofType]; !ok {
		return nil, errors.New("unmarshaler is not registered")
	}

	if _, ok := f.topics[proofType]; !ok {
		topic, err := f.pubsub.Join(getSubTopic(proofType))
		if err != nil {
			return nil, err
		}
		log.Debugf("successfully subscibed to topic", topic.String())
		f.topics[proofType] = topic
	}

	return newSubscription(f.topics[proofType], f.unmarshalers[proofType])
}

func (f *service) RegisterUnmarshaler(proofType ProofType, u proofUnmarshaler) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.unmarshalers[proofType]; ok {
		return errors.New("unmarshaler is registered")
	}
	f.unmarshalers[proofType] = u

	return nil
}

func (f *service) UnregisterUnmarshaler(proofType ProofType) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.unmarshalers[proofType]; !ok {
		return errors.New("unmarshaler is not registered")
	}
	delete(f.unmarshalers, proofType)

	return nil

}

func (f *service) Broadcast(ctx context.Context, p Proof) error {
	bin, err := p.MarshalBinary()
	if err != nil {
		return err
	}
	if topic, ok := f.topics[p.Type()]; ok {
		return topic.Publish(ctx, bin)
	}

	return errors.New("topic is not found")
}

func (f *service) AddValidator(proofType ProofType, val Validator) error {
	topic := getSubTopic(proofType)
	pval := func(ctx context.Context, _ peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		return val(ctx, proofType, msg.Data)
	}

	return f.pubsub.RegisterTopicValidator(topic, pval)
}

func (f *service) processIncoming(ctx context.Context, proofType ProofType, data []byte) pubsub.ValidationResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	unmarshaler, ok := f.unmarshalers[proofType]
	if !ok {
		log.Error("unmarshaler is not found")
		return pubsub.ValidationReject
	}
	proof, err := unmarshaler(data)
	if err != nil {
		log.Errorw("unmarshalling header error", err)
		return pubsub.ValidationReject
	}

	extHeader, err := f.getter(ctx, proof.Height())
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
	log.Debugw("received Bad Encoding Fraud Proof from block ", "hash", hex.EncodeToString(extHeader.DAH.Hash()))
	return pubsub.ValidationAccept
}

func getSubTopic(p ProofType) string {
	switch p {
	case BadEncoding:
		return "befp-sub"
	default:
		panic(fmt.Sprintf("invalid proof type %d", p))
	}
}
