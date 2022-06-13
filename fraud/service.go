package fraud

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

const (
	fetchHeaderTimeout = time.Minute * 2
)

// service is responsible for validating and propagating Fraud Proofs.
// It implements the Service interface.
type service struct {
	topicsLk sync.RWMutex
	topics   map[ProofType]*topic

	pubsub *pubsub.PubSub
	getter headerFetcher
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
	f.topicsLk.Lock()
	t, ok := f.topics[proofType]
	f.topicsLk.Unlock()
	if !ok {
		return nil, errors.New("fraud: unmarshaler for the given proof type is not registered")
	}
	return newSubscription(t)
}

func (f *service) RegisterUnmarshaler(proofType ProofType, u ProofUnmarshaler) error {
	f.topicsLk.Lock()
	defer f.topicsLk.Unlock()

	t, err := f.pubsub.Join(getSubTopic(proofType))
	if err != nil {
		return err
	}
	err = f.pubsub.RegisterTopicValidator(
		getSubTopic(proofType),
		func(ctx context.Context, _ peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			return f.processIncoming(ctx, proofType, msg)
		},
		// make validation synchronous.
		pubsub.WithValidatorInline(true),
	)
	if err != nil {
		return err
	}

	f.topics[proofType] = &topic{topic: t, unmarshal: u}
	return nil
}

func (f *service) UnregisterUnmarshaler(proofType ProofType) error {
	f.topicsLk.Lock()
	defer f.topicsLk.Unlock()
	t, ok := f.topics[proofType]
	if !ok {
		return errors.New("fraud: unmarshaler for the given proof type is not registered")
	}
	delete(f.topics, proofType)
	return t.close()

}

func (f *service) Broadcast(ctx context.Context, p Proof) error {
	bin, err := p.MarshalBinary()
	if err != nil {
		return err
	}
	f.topicsLk.RLock()
	t, ok := f.topics[p.Type()]
	f.topicsLk.RUnlock()
	if !ok {
		return errors.New("fraud: unmarshaler for the given proof type is not registered")
	}
	return t.publish(ctx, bin)
}

func (f *service) processIncoming(
	ctx context.Context,
	proofType ProofType,
	msg *pubsub.Message,
) pubsub.ValidationResult {
	f.topicsLk.RLock()
	t, ok := f.topics[proofType]
	f.topicsLk.RUnlock()
	if !ok {
		panic("fraud: unmarshaler for the given proof type is not registered")
	}
	proof, err := t.unmarshal(msg.Data)
	if err != nil {
		log.Errorw("failed to unmarshal fraud proof", err)
		f.pubsub.BlacklistPeer(msg.ReceivedFrom)
		return pubsub.ValidationReject
	}

	// create a timeout for block fetching since our validator will be called synchronously
	// and getter is a blocking function.
	newCtx, cancel := context.WithTimeout(ctx, fetchHeaderTimeout)
	extHeader, err := f.getter(newCtx, proof.Height())
	defer cancel()
	if err != nil {
		// Timeout means there is a problem with the network.
		// As we cannot prove or discard Fraud Proof, user must restart the node.
		if errors.Is(err, context.DeadlineExceeded) {
			log.Errorw("failed to fetch header. Timeout reached.")
			// TODO(@vgonkivs): add handling for this case. As we are not able to verify fraud proof.
		}
		log.Errorw("failed to fetch header to verify a fraud proof",
			"err", err, "proofType", proof.Type(), "height", proof.Height())
		return pubsub.ValidationIgnore
	}
	err = proof.Validate(extHeader)
	if err != nil {
		log.Errorw("proof validation err: ",
			"err", err, "proofType", proof.Type(), "height", proof.Height())
		f.pubsub.BlacklistPeer(msg.ReceivedFrom)
		return pubsub.ValidationReject
	}
	log.Warnw("received fraud proof", "proofType", proof.Type(),
		"hash", hex.EncodeToString(extHeader.DAH.Hash()),
		"from", msg.ReceivedFrom.String(),
	)
	return pubsub.ValidationAccept
}

func getSubTopic(p ProofType) string {
	return p.String() + "-sub"
}
