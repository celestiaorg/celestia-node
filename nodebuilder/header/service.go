package header

import (
	"context"
	"fmt"

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
	// TODO(vgonkivs): remove after https://github.com/celestiaorg/go-header/issues/32 will be
	// implemented
	head, err := s.syncer.Head(ctx)
	if err != nil {
		return nil, err
	}

	if uint64(head.Height()) < height {
		return nil, fmt.Errorf("header: given height exceeds network head."+
			"networkHeight:%d, requestedHeight:%d", head.Height(), height)
	}
	if uint64(head.Height()) == height {
		return head, nil
	}

	if !s.store.HasAt(ctx, height) {
		return nil, fmt.Errorf("header: given height exceeds local head. requestedHeight:%d", height)
	}
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

func (s *Service) Subscribe(ctx context.Context) (<-chan *header.ExtendedHeader, error) {
	subscription, err := s.sub.Subscribe()
	if err != nil {
		return nil, err
	}

	headerCh := make(chan *header.ExtendedHeader)
	go func() {
		defer close(headerCh)
		defer subscription.Cancel()

		for {
			h, err := subscription.NextHeader(ctx)
			if err != nil {
				if err != context.DeadlineExceeded && err != context.Canceled {
					log.Errorw("fetching header from subscription", "err", err)
				}
				return
			}

			select {
			case <-ctx.Done():
				return
			case headerCh <- h:
			}
		}
	}()
	return headerCh, nil
}
