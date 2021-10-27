package header

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// Service represents the Header service that can be started / stopped on a node.
// Service contains 3 main functionalities:
// 		1. Listening for/requesting new ExtendedHeaders from the network.
// 		2. Verifying and serving ExtendedHeaders to the network.
// 		3. Storing/caching ExtendedHeaders.
type Service struct {
	exchange Exchange
	store    Store

	topic  *pubsub.Topic // instantiated header-sub topic
	pubsub *pubsub.PubSub
}

var log = logging.Logger("header-service")

// NewHeaderService creates a new instance of header Service.
func NewHeaderService(exchange Exchange, store Store, pubsub *pubsub.PubSub) *Service {
	return &Service{
		exchange: exchange,
		store:    store,
		pubsub:   pubsub,
	}
}

// Start starts the header Service.
func (s *Service) Start(ctx context.Context) error {
	log.Info("starting header service")

	topic, err := s.pubsub.Join(ExtendedHeaderSubTopic)
	if err != nil {
		return err
	}
	s.topic = topic

	// TODO @renaynay: start internal header service processes
	return nil
}

// Subscribe returns a new subscription to the header pubsub topic
func (s *Service) Subscribe() (Subscription, error) {
	if s.topic == nil {
		return nil, fmt.Errorf("header topic is not instantiated, service must be started before subscribing")
	}

	return newSubscription(s.topic)
}

// Stop stops the header Service.
func (s *Service) Stop(ctx context.Context) error {
	log.Info("stopping header service")

	return s.topic.Close()
}
