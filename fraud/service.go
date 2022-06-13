package fraud

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// service is responsible for validating and propagating Fraud Proofs.
// It implements the Service interface.
type service struct {
	pubsub *pubsub.PubSub
	getter headerFetcher

	mu     sync.RWMutex
	topics map[ProofType]*topic
}

func NewService(p *pubsub.PubSub, getter headerFetcher) Service {
	return &service{
		pubsub: p,
		getter: getter,
		topics: make(map[ProofType]*topic),
	}
}

func (f *service) Subscribe(proofType ProofType) (Subscription, error) {
	// TODO: @vgonkivs check if fraud proof is in fraud store, then return with error
	f.mu.Lock()
	t, ok := f.topics[proofType]
	f.mu.Unlock()
	if !ok {
		return nil, errors.New("fraud: topic was not created")
	}
	return newSubscription(t)
}

func (f *service) RegisterUnmarshaler(proofType ProofType, u ProofUnmarshaler) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	t, err := createTopic(f.pubsub, proofType, u, f.processIncoming)
	if err != nil {
		return err
	}
	f.topics[proofType] = t
	return nil
}

func (f *service) UnregisterUnmarshaler(proofType ProofType) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.topics[proofType]
	if !ok {
		return errors.New("fraud: topic was not created")
	}
	delete(f.topics, proofType)
	return t.close()

}

func (f *service) Broadcast(ctx context.Context, p Proof) error {
	bin, err := p.MarshalBinary()
	if err != nil {
		return err
	}
	f.mu.RLock()
	t, ok := f.topics[p.Type()]
	f.mu.RUnlock()
	if !ok {
		return errors.New("fraud: topic was not created")
	}
	return t.publish(ctx, bin)
}

func (f *service) processIncoming(
	ctx context.Context,
	proofType ProofType,
	msg *pubsub.Message,
) pubsub.ValidationResult {
	f.mu.RLock()
	t, ok := f.topics[proofType]
	f.mu.RUnlock()
	if !ok {
		log.Error("topic was not created")
		return pubsub.ValidationReject
	}
	proof, err := t.unmarshal(msg.Data)
	if err != nil {
		log.Errorw("unmarshaling header error", err)
		return pubsub.ValidationReject
	}

	// create a timeout for block fetching since our validator will be called synchronously
	// and getter is a blocking function.
	newCtx, cancel := context.WithTimeout(ctx, time.Minute*2)
	extHeader, err := f.getter(newCtx, proof.Height())
	if err != nil {
		cancel()
		// Timeout means there is a problem with the network.
		// As we cannot prove or discard Fraud Proof, user must restart the node.
		if errors.Is(err, context.DeadlineExceeded) {
			log.Fatal("could not get block. Timeout reached. Please restart your node.")
		}
		log.Errorw("failed to fetch block: ",
			"err", err)
		return pubsub.ValidationReject
	}
	defer cancel()
	err = proof.Validate(extHeader)
	if err != nil {
		log.Errorw("validation err: ",
			"err", err)
		f.pubsub.BlacklistPeer(msg.ReceivedFrom)
		return pubsub.ValidationReject
	}
	log.Warnw("received an inbound proof", "kind", proof.Type(),
		"hash", hex.EncodeToString(extHeader.DAH.Hash()),
		"from", msg.ReceivedFrom.String(),
	)
	return pubsub.ValidationAccept
}

func getSubTopic(p ProofType) string {
	return p.String() + "-sub"
}
