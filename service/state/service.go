package state

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/sync"
	"github.com/celestiaorg/nmt/namespace"
)

// Service can access state-related information via the given
// Accessor.
type Service struct {
	ctx    context.Context
	cancel context.CancelFunc

	accessor Accessor

	getter header.Getter
	sync   sync.Sub
}

// NewService constructs a new state Service.
func NewService(accessor Accessor, getter header.Getter, sync sync.Sub) *Service {
	return &Service{
		accessor: accessor,
		getter:   getter,
		sync:     sync,
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
	if !s.sync.Finished() {
		return nil, fmt.Errorf("node is not synced up to network head yet, balances will not be current")
	}
	return s.accessor.BalanceForAddress(ctx, addr)
}

func (s *Service) SubmitTx(ctx context.Context, tx Tx) (*TxResponse, error) {
	return s.accessor.SubmitTx(ctx, tx)
}

func (s *Service) Transfer(ctx context.Context, to Address, amount Int, gasLimit uint64) (*TxResponse, error) {
	return s.accessor.Transfer(ctx, to, amount, gasLimit)
}

func (s *Service) Start(context.Context) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return nil
}

func (s *Service) Stop(context.Context) error {
	s.cancel()
	return nil
}

// IsStopped checks if context was canceled.
func (s *Service) IsStopped() bool {
	return s.ctx.Err() != nil
}
