package state

import (
	"context"
	"sync/atomic"

	"github.com/celestiaorg/nmt/namespace"

	"github.com/celestiaorg/celestia-node/fraud"
)

// Service can access state-related information via the given
// Accessor.
type Service struct {
	accessor Accessor

	fsub   fraud.Subscriber
	cancel context.CancelFunc

	befpReceived uint64
}

// NewService constructs a new state Service.
func NewService(accessor Accessor, fSub fraud.Subscriber) *Service {
	return &Service{
		accessor: accessor,
		fsub:     fSub,
	}
}

func (s *Service) SubmitPayForData(
	ctx context.Context,
	nID namespace.ID,
	data []byte,
	gasLim uint64,
) (*TxResponse, error) {
	return s.accessor.SubmitPayForData(ctx, nID, data, gasLim)
}

func (s *Service) Balance(ctx context.Context) (*Balance, error) {
	return s.accessor.Balance(ctx)
}

func (s *Service) BalanceForAddress(ctx context.Context, addr Address) (*Balance, error) {
	return s.accessor.BalanceForAddress(ctx, addr)
}

func (s *Service) SubmitTx(ctx context.Context, tx Tx) (*TxResponse, error) {
	return s.accessor.SubmitTx(ctx, tx)
}

func (s *Service) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go fraud.SubscribeToBEFP(ctx, s.fsub, func(context.Context) error {
		atomic.StoreUint64(&s.befpReceived, 1)
		return nil
	})
	return nil
}

func (s *Service) Stop(context.Context) error {
	s.cancel()
	s.cancel = nil
	return nil
}

func (s *Service) BEFPReceived() bool {
	return atomic.LoadUint64(&s.befpReceived) == 1
}
