package header

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
)

var tracer = otel.Tracer("header/service")

// ErrHeightZero returned when the provided block height is equal to 0.
var ErrHeightZero = errors.New("height is equal to 0")

// Service represents the header Service that can be started / stopped on a node.
// Service's main function is to manage its sub-services. Service can contain several
// sub-services, such as Exchange, ExchangeServer, Syncer, and so forth.
type Service struct {
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
	// getting Syncer wrapped in ServiceBreaker so we ensure service breaker is constructed
	syncer *modfraud.ServiceBreaker[*sync.Syncer[*header.ExtendedHeader], *header.ExtendedHeader],
	sub libhead.Subscriber[*header.ExtendedHeader],
	p2pServer *p2p.ExchangeServer[*header.ExtendedHeader],
	store libhead.Store[*header.ExtendedHeader],
) Module {
	return &Service{
		syncer:    syncer.Service,
		sub:       sub,
		p2pServer: p2pServer,
		store:     store,
	}
}

func (s *Service) GetByHash(ctx context.Context, hash libhead.Hash) (_ *header.ExtendedHeader, err error) {
	ctx, span := tracer.Start(ctx, "header/get-by-hash")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("getting header by hash", "hash", hash, "err", err)
		}
	}()
	log.Info("getting header by hash", "hash", hash)

	return s.store.Get(ctx, hash)
}

func (s *Service) GetRangeByHeight(
	ctx context.Context,
	from *header.ExtendedHeader,
	to uint64,
) (_ []*header.ExtendedHeader, err error) {
	ctx, span := tracer.Start(ctx, "header/get-range-by-height")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("getting header range by height", "from", from.Height(), "to", to, "err", err)
		}
	}()
	log.Infow("getting header range by height", "from", from.Height(), "to", to)

	return s.store.GetRangeByHeight(ctx, from, to)
}

func (s *Service) GetByHeight(ctx context.Context, height uint64) (_ *header.ExtendedHeader, err error) {
	if height == 0 {
		return nil, ErrHeightZero
	}

	ctx, span := tracer.Start(ctx, "header/get-by-height")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("getting header by height", "height", height, "err", err)
		}
	}()
	log.Infow("getting header by height", "height", height)

	head, err := s.syncer.Head(ctx)
	switch {
	case err != nil:
		return nil, fmt.Errorf("syncer head: %w", err)
	case head.Height() == height:
		return head, nil
	case head.Height()+1 < height:
		return nil, fmt.Errorf("header: given height is from the future: "+
			"networkHeight: %d, requestedHeight: %d", head.Height(), height)
	}

	tail, err := s.store.Tail(ctx)
	switch {
	case err != nil:
		return nil, fmt.Errorf("store tail: %w", err)
	case height < tail.Height():
		log.Warnf(`requested header (%d) is below Tail (%d)
		 	lazy fetching (https://github.com/celestiaorg/go-header/issues/334) is not currently supported
			make sure to set SyncFromHeight value in config covering desired header height`, height, tail.Height())
		return nil, fmt.Errorf("requested header (%d) is below Tail (%d)", height, tail.Height())
	case height == tail.Height():
		return tail, nil
	}

	head, err = s.store.Head(ctx)
	switch {
	case err != nil:
		return nil, fmt.Errorf("store head: %w", err)
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

func (s *Service) WaitForHeight(ctx context.Context, height uint64) (_ *header.ExtendedHeader, err error) {
	ctx, span := tracer.Start(ctx, "header/wait-for-height")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("waiting for header", "height", height, "err", err)
		}
	}()
	log.Infow("awaiting header", "height", height)

	return s.store.GetByHeight(ctx, height)
}

func (s *Service) LocalHead(ctx context.Context) (_ *header.ExtendedHeader, err error) {
	ctx, span := tracer.Start(ctx, "header/local-head")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("getting local head", "err", err)
		}
	}()
	log.Info("getting local head")

	return s.store.Head(ctx)
}

func (s *Service) SyncState(context.Context) (sync.State, error) {
	return s.syncer.State(), nil
}

func (s *Service) SyncWait(ctx context.Context) (err error) {
	ctx, span := tracer.Start(ctx, "header/sync-wait")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("awaiting synchronization to finish", "err", err)
		}
	}()
	log.Info("awaiting synchronization to finish")

	return s.syncer.SyncWait(ctx)
}

func (s *Service) NetworkHead(ctx context.Context) (_ *header.ExtendedHeader, err error) {
	ctx, span := tracer.Start(ctx, "header/network-head")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("getting network head", "err", err)
		}
	}()
	log.Info("getting network head")

	return s.syncer.Head(ctx)
}

func (s *Service) Tail(ctx context.Context) (_ *header.ExtendedHeader, err error) {
	ctx, span := tracer.Start(ctx, "header/tail")
	defer func() {
		utils.SetStatusAndEnd(span, err)
		if err != nil {
			log.Errorw("getting tail header", "err", err)
		}
	}()
	log.Info("getting tail header")

	return s.store.Tail(ctx)
}

func (s *Service) Subscribe(ctx context.Context) (<-chan *header.ExtendedHeader, error) {
	log.Info("subscribing to headers")
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
				if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
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
