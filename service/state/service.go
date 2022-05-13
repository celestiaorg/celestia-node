package state

import (
	"context"

	"github.com/celestiaorg/nmt/namespace"
)

// Service can access state-related information via the given
// Accessor.
type Service struct {
	accessor Accessor
}

// NewService constructs a new state Service.
func NewService(accessor Accessor) *Service {
	return &Service{
		accessor: accessor,
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
