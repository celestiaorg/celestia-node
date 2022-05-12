package header

import (
	"context"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/p2p"
	"github.com/celestiaorg/celestia-node/header/sync"
)

var log = logging.Logger("service/header")

// Service represents the header service that can be started / stopped on a node.
// Service's main function is to manage its sub-services. Service can contain several
// sub-services, such as Exchange, ExchangeServer, Syncer, and so forth.
type Service struct {
	ex header.Exchange

	syncer    *sync.Syncer
	sub       header.Subscriber
	p2pServer *p2p.ExchangeServer
	store     header.Store
}

// NewHeaderService creates a new instance of header Service.
func NewHeaderService(
	syncer *sync.Syncer,
	sub header.Subscriber,
	p2pServer *p2p.ExchangeServer,
	ex header.Exchange,
	store header.Store) *Service {
	return &Service{
		syncer:    syncer,
		sub:       sub,
		p2pServer: p2pServer,
		ex:        ex,
		store:     store,
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
func (s *Service) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	return s.store.GetByHeight(ctx, height)
}

// Head returns the ExtendedHeader of the chain head.
func (s *Service) Head(ctx context.Context) (*header.ExtendedHeader, error) {
	return s.store.Head(ctx)
}

// IsSyncing returns the status of sync
func (s *Service) IsSyncing() bool {
	return !s.syncer.State().Finished()
}
