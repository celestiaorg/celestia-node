package header

import (
	"context"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
)

// Service represents the header Service that can be started / stopped on a node.
// Service's main function is to manage its sub-services. Service can contain several
// sub-services, such as Exchange, ExchangeServer, Syncer, and so forth.
type Service struct {
	ex libhead.Exchange[*header.ExtendedHeader]

	syncer    *sync.Syncer[*header.ExtendedHeader]
	sub       libhead.Subscriber[*header.ExtendedHeader]
	p2pServer *p2p.ExchangeServer[*header.ExtendedHeader]
	store     libhead.Store[*header.ExtendedHeader]
}

// newHeaderService creates a new instance of header Service.
func newHeaderService(
	syncer *sync.Syncer[*header.ExtendedHeader],
	sub libhead.Subscriber[*header.ExtendedHeader],
	p2pServer *p2p.ExchangeServer[*header.ExtendedHeader],
	ex libhead.Exchange[*header.ExtendedHeader],
	store libhead.Store[*header.ExtendedHeader]) Module {
	return &Service{
		syncer:    syncer,
		sub:       sub,
		p2pServer: p2pServer,
		ex:        ex,
		store:     store,
	}
}

func (s *Service) GetByHash(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	return s.store.Get(ctx, hash)
}

func (s *Service) GetVerifiedRangeByHeight(
	ctx context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	return s.store.GetVerifiedRange(ctx, from, to)
}

func (s *Service) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	return s.store.GetByHeight(ctx, height)
}

func (s *Service) LocalHead(ctx context.Context) (*header.ExtendedHeader, error) {
	return s.store.Head(ctx)
}

func (s *Service) SyncState(context.Context) (sync.State, error) {
	return s.syncer.State(), nil
}

func (s *Service) SyncWait(ctx context.Context) error {
	return s.syncer.SyncWait(ctx)
}

func (s *Service) NetworkHead(ctx context.Context) (*header.ExtendedHeader, error) {
	return s.syncer.Head(ctx)
}
