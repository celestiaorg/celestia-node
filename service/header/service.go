package header

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("header")

// Service represents the header service that can be started / stopped on a node.
// Service's main function is to manage its sub-services. Service can contain several
// sub-services, such as Exchange, P2PExchangeServer, Syncer, and so forth.
type Service struct {
	ex Exchange

	syncer    *Syncer
	sub       Subscriber
	p2pServer *P2PExchangeServer
}

// NewHeaderService creates a new instance of header Service.
func NewHeaderService(
	syncer *Syncer,
	sub Subscriber,
	p2pServer *P2PExchangeServer,
	ex Exchange) *Service {
	return &Service{
		syncer:    syncer,
		sub:       sub,
		p2pServer: p2pServer,
		ex:        ex,
	}
}

// Start starts the header Service.
func (s *Service) Start(context.Context) error {
	log.Info("starting header service")
	return nil
}

// Stop stops the header Service.
func (s *Service) Stop(context.Context) error {
	log.Info("stopping header service")
	return nil
}

// GetByHeight returns the ExtendedHeader at the given height, blocking
// until header has been processed by the store or context deadline is exceeded.
func (s *Service) GetByHeight(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	return s.syncer.store.GetByHeight(ctx, height)
}

// IsSyncing returns the status of sync
func (s *Service) IsSyncing() bool {
	return !s.syncer.State().Finished()
}
