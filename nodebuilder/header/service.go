package header

import (
	"context"
	"errors"
	"fmt"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
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
	// getting Syncer wrapped in ServiceBreaker so we ensure service breaker is constructed
	syncer *modfraud.ServiceBreaker[*sync.Syncer[*header.ExtendedHeader], *header.ExtendedHeader],
	sub libhead.Subscriber[*header.ExtendedHeader],
	p2pServer *p2p.ExchangeServer[*header.ExtendedHeader],
	ex libhead.Exchange[*header.ExtendedHeader],
	store libhead.Store[*header.ExtendedHeader],
) Module {
	return &Service{
		syncer:    syncer.Service,
		sub:       sub,
		p2pServer: p2pServer,
		ex:        ex,
		store:     store,
	}
}

func (s *Service) GetByHash(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	return s.store.Get(ctx, hash)
}

func (s *Service) GetRangeByHeight(
	ctx context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	return s.store.GetRangeByHeight(ctx, from, to)
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

// GetDataCommitment collects the data roots over a provided ordered range of blocks,
// and then creates a new Merkle root of those data roots. The range is end exclusive.
func (s *Service) GetDataCommitment(ctx context.Context, start, end uint64) (*DataCommitment, error) {
	log.Debugw("validating the data commitment range", "start", start, "end", end)
	err := s.validateDataCommitmentRange(ctx, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("fetching the data root tuples", "start", start, "end", end)
	tuples, err := s.fetchDataRootTuples(ctx, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("hashing the data root tuples", "start", start, "end", end)
	root, err := hashDataRootTuples(tuples)
	if err != nil {
		return nil, err
	}
	// Create data commitment
	dataCommitment := DataCommitment(root)
	return &dataCommitment, nil
}

// GetDataRootInclusionProof creates an inclusion proof for the data root of block
// height `height` in the set of blocks defined by `start` and `end`. The range
// is end exclusive.
func (s *Service) GetDataRootInclusionProof(
	ctx context.Context,
	height int64,
	start,
	end uint64,
) (*DataRootTupleInclusionProof, error) {
	log.Debugw(
		"validating the data root inclusion proof request",
		"start",
		start,
		"end",
		end,
		"height",
		height,
	)
	err := s.validateDataRootInclusionProofRequest(ctx, uint64(height), start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("fetching the data root tuples", "start", start, "end", end)
	tuples, err := s.fetchDataRootTuples(ctx, start, end)
	if err != nil {
		return nil, err
	}
	log.Debugw("proving the data root tuples", "start", start, "end", end)
	proof, err := proveDataRootTuples(tuples, height)
	if err != nil {
		return nil, err
	}
	dataRootTupleInclusionProof := DataRootTupleInclusionProof(proof)
	return &dataRootTupleInclusionProof, nil
}
