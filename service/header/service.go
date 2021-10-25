package header

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
)

// Service represents the Header service that can be started / stopped on a node.
// Service contains 3 main functionalities:
// 		1. Listening for/requesting new ExtendedHeaders from the network.
// 		2. Verifying and serving ExtendedHeaders to the network.
// 		3. Storing/caching ExtendedHeaders.
type Service struct {
	exchange Exchange
	store    Store
}

var log = logging.Logger("header-service")

// NewHeaderService creates a new instance of header Service.
func NewHeaderService(exchange Exchange, store Store) *Service {
	return &Service{
		exchange: exchange,
		store:    store,
	}
}

// Start starts the header Service.
func (s *Service) Start(ctx context.Context) error {
	log.Info("starting header service")
	return nil
}

// Stop stops the header Service.
func (s *Service) Stop(ctx context.Context) error {
	log.Info("stopping header service")
	return nil
}
