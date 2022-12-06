package header

import (
	"context"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/p2p"
	"github.com/celestiaorg/celestia-node/header/sync"
)

// Service represents the header Service that can be started / stopped on a node.
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
	store header.Store) Module {
	return &Service{
		syncer:    syncer,
		sub:       sub,
		p2pServer: p2pServer,
		ex:        ex,
		store:     store,
	}
}

func (s *Service) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	return s.store.GetByHeight(ctx, height)
}

func (s *Service) Head(ctx context.Context) (*header.ExtendedHeader, error) {
	return s.store.Head(ctx)
}

func (s *Service) IsSyncing(context.Context) bool {
	return !s.syncer.State().Finished()
}
