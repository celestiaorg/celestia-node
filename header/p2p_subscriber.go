package header

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/minio/blake2b-simd"
)

// PubSubTopic hardcodes the name of the ExtendedHeader
// gossipsub topic.
const PubSubTopic = "header-sub"

// P2PSubscriber manages the lifecycle and relationship of header Service
// with the "header-sub" gossipsub topic.
type P2PSubscriber struct {
	pubsub *pubsub.PubSub
	topic  *pubsub.Topic
}

// NewP2PSubscriber returns a P2PSubscriber that manages the header Service's
// relationship with the "header-sub" gossipsub topic.
func NewP2PSubscriber(ps *pubsub.PubSub) *P2PSubscriber {
	return &P2PSubscriber{
		pubsub: ps,
	}
}

// Start starts the P2PSubscriber, registering a topic validator for the "header-sub"
// topic and joining it.
func (p *P2PSubscriber) Start(context.Context) (err error) {
	p.topic, err = p.pubsub.Join(PubSubTopic, pubsub.WithTopicMessageIdFn(msgID))
	return err
}

// Stop closes the topic and unregisters its validator.
func (p *P2PSubscriber) Stop(context.Context) error {
	err := p.pubsub.UnregisterTopicValidator(PubSubTopic)
	if err != nil {
		log.Warnf("unregistering validator: %s", err)
	}

	return p.topic.Close()
}

// AddValidator applies basic pubsub validator for the topic.
func (p *P2PSubscriber) AddValidator(val Validator) error {
	pval := func(ctx context.Context, p peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		maybeHead, err := UnmarshalExtendedHeader(msg.Data)
		if err != nil {
			log.Errorw("unmarshalling header",
				"from", p.ShortString(),
				"err", err)
			return pubsub.ValidationReject
		}

		return val(ctx, maybeHead)
	}
	return p.pubsub.RegisterTopicValidator(PubSubTopic, pval)
}

// Subscribe returns a new subscription to the P2PSubscriber's
// topic.
func (p *P2PSubscriber) Subscribe() (Subscription, error) {
	if p.topic == nil {
		return nil, fmt.Errorf("header topic is not instantiated, service must be started before subscribing")
	}

	return newSubscription(p.topic)
}

// Broadcast broadcasts the given ExtendedHeader to the topic.
func (p *P2PSubscriber) Broadcast(ctx context.Context, header *ExtendedHeader, opts ...pubsub.PubOpt) error {
	bin, err := header.MarshalBinary()
	if err != nil {
		return err
	}
	return p.topic.Publish(ctx, bin, opts...)
}

// msgID computes an id for a pubsub message
// TODO(@Wondertan): This cause additional allocations per each recvd message in the topic. Find a way to avoid those.
func msgID(pmsg *pb.Message) string {
	mID := func(data []byte) string {
		hash := blake2b.Sum256(data)
		return string(hash[:])
	}

	h, err := UnmarshalExtendedHeader(pmsg.Data)
	if err != nil {
		// There is nothing we can do about the error, and it will be anyway caught during validation.
		// We also *have* to return some ID for the msg, so give the hash of even faulty msg
		return mID(pmsg.Data)
	}

	// IMPORTANT NOTE:
	// Due to the nature of the Tendermint consensus, validators don't necessarily collect commit signatures from the
	// entire validator set, but only the minimum required amount of them (>2/3 of voting power). In addition,
	// signatures are collected asynchronously. Therefore, each validator may have a different set of signatures that
	// pass the minimum required voting power threshold, causing nondeterminism in the header message gossiped over the
	// network. Subsequently, this causes message duplicates as each Bridge Node, connected to a personal validator,
	// sends the validator's own view of commits of effectively the same header.
	//
	// To solve the problem above, we exclude nondeterministic value from message id calculation
	h.Commit.Signatures = nil

	data, err := MarshalExtendedHeader(h)
	if err != nil {
		// See the note under unmarshalling step
		return mID(pmsg.Data)
	}

	return mID(data)
}
