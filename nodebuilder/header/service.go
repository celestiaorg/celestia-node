package header

import (
	"context"
	headerpkg "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/libs/header/p2p"
	"github.com/celestiaorg/celestia-node/libs/header/sync"

	"github.com/celestiaorg/celestia-node/header"
)

// Service represents the header Service that can be started / stopped on a node.
// Service's main function is to manage its sub-services. Service can contain several
// sub-services, such as Exchange, ExchangeServer, Syncer, and so forth.
type Service struct {
	ex headerpkg.Exchange[*header.ExtendedHeader]

	syncer    *sync.Syncer[*header.ExtendedHeader]
	sub       headerpkg.Subscriber[*header.ExtendedHeader]
	p2pServer *p2p.ExchangeServer[*header.ExtendedHeader]
	store     headerpkg.Store[*header.ExtendedHeader]
}

// NewHeaderService creates a new instance of header Service.
func NewHeaderService(
	syncer *sync.Syncer[*header.ExtendedHeader],
	sub headerpkg.Subscriber[*header.ExtendedHeader],
	p2pServer *p2p.ExchangeServer[*header.ExtendedHeader],
	ex headerpkg.Exchange[*header.ExtendedHeader],
	store headerpkg.Store[*header.ExtendedHeader]) Module {
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
