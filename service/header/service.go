package header

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

var log = logging.Logger("header-service")

// PubSubTopic hardcodes the name of the ExtendedHeader
// gossipsub topic.
const PubSubTopic = "header-sub"

// Service represents the Header service that can be started / stopped on a node.
// Service contains 3 main functionalities:
// 		1. Listening for/requesting new ExtendedHeaders from the network.
// 		2. Verifying and serving ExtendedHeaders to the network.
// 		3. Storing/caching ExtendedHeaders.
type Service struct {
	syncer *Syncer

	topic  *pubsub.Topic // instantiated header-sub topic
	pubsub *pubsub.PubSub

	ctx    context.Context
	cancel context.CancelFunc
}

// NewHeaderService creates a new instance of header Service.
func NewHeaderService(
	syncer *Syncer,
	pubsub *pubsub.PubSub,
) *Service {
	return &Service{
		syncer: syncer,
		pubsub: pubsub,
	}
}

// Start starts the header Service.
func (s *Service) Start(ctx context.Context) error {
	if s.cancel != nil {
		return fmt.Errorf("header: Service already started")
	}
	log.Info("starting header service")

	s.ctx, s.cancel = context.WithCancel(context.Background())
	go s.syncer.Sync(s.ctx)

	err := s.pubsub.RegisterTopicValidator(PubSubTopic, s.syncer.Validate)
	if err != nil {
		return err
	}

	s.topic, err = s.pubsub.Join(PubSubTopic)
	return err
}

// Stop stops the header Service.
func (s *Service) Stop(context.Context) error {
	if s.cancel == nil {
		return fmt.Errorf("header: Service already stopped")
	}
	log.Info("stopping header service")

	s.cancel()
	err := s.pubsub.UnregisterTopicValidator(PubSubTopic)
	if err != nil {
		return err
	}

	return s.topic.Close()
}

// Subscribe returns a new subscription to the header pubsub topic
func (s *Service) Subscribe() (Subscription, error) {
	if s.topic == nil {
		return nil, fmt.Errorf("header topic is not instantiated, service must be started before subscribing")
	}

	return newSubscription(s.topic)
}

func (s *Service) Broadcast(ctx context.Context, header *ExtendedHeader) error {
	bin, err := header.MarshalBinary()
	if err != nil {
		return err
	}

	return s.topic.Publish(ctx, bin)
}
