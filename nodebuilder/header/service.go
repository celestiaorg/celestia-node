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

	syncer    syncer
	sub       libhead.Subscriber[*header.ExtendedHeader]
	p2pServer *p2p.ExchangeServer[*header.ExtendedHeader]
	store     libhead.Store[*header.ExtendedHeader]
}

// syncer bare minimum Syncer interface for testing
type syncer interface {
	libhead.Head[*header.ExtendedHeader]

	State() sync.State
	SyncWait(ctx context.Context) error
}

// newHeaderService creates a new instance of header Service.
func newHeaderService(
	syncer *sync.Syncer[*header.ExtendedHeader],
	sub libhead.Subscriber[*header.ExtendedHeader],
	p2pServer *p2p.ExchangeServer[*header.ExtendedHeader],
	ex libhead.Exchange[*header.ExtendedHeader],
	store libhead.Store[*header.ExtendedHeader],
) Module {
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
	head, err := s.syncer.Head(ctx)
	switch {
	case err != nil:
		return nil, err
	case head.Height() == height:
		return head, nil
	case head.Height()+1 < height:
		return nil, fmt.Errorf("header: given height is from the future: "+
			"networkHeight: %d, requestedHeight: %d", head.Height(), height)
	}

	// TODO(vgonkivs): remove after https://github.com/celestiaorg/go-header/issues/32 is
	//  implemented and fetch header from HeaderEx if missing locally
	head, err = s.store.Head(ctx)
	switch {
	case err != nil:
		return nil, err
	case head.Height() == height:
		return head, nil
	// `+1` allows for one header network lag, e.g. user request header that is milliseconds away
	case head.Height()+1 < height:
		return nil, fmt.Errorf("header: syncing in progress: "+
			"localHeadHeight: %d, requestedHeight: %d", head.Height(), height)
	default:
		return s.store.GetByHeight(ctx, height)
	}
}

func (s *Service) WaitForHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
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
