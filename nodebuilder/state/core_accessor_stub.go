package state

import (
	"context"
	"errors"

	"github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
)

var ErrNoStateAccess = errors.New("node is running without state access")

// stubbedCoreAccessor provides a stub for the state module to return
// errors when state endpoints are accessed without a running connection
// to a core endpoint.
type stubbedCoreAccessor struct{}

func (c stubbedCoreAccessor) IsStopped(context.Context) bool {
	return true
}

func (c stubbedCoreAccessor) AccountAddress(context.Context) (state.Address, error) {
	return state.Address{}, ErrNoStateAccess
}

func (c stubbedCoreAccessor) Balance(context.Context) (*state.Balance, error) {
	return nil, ErrNoStateAccess
}

func (c stubbedCoreAccessor) BalanceForAddress(
	context.Context,
	state.Address,
) (*state.Balance, error) {
	return nil, ErrNoStateAccess
}

func (c stubbedCoreAccessor) Transfer(
	_ context.Context,
	_ state.AccAddress,
	_, _ state.Int,
	_ uint64,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (c stubbedCoreAccessor) SubmitTx(context.Context, state.Tx) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (c stubbedCoreAccessor) SubmitPayForBlob(
	context.Context,
	state.Int,
	uint64,
	[]*blob.Blob,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (c stubbedCoreAccessor) CancelUnbondingDelegation(
	_ context.Context,
	_ state.ValAddress,
	_, _, _ state.Int,
	_ uint64,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (c stubbedCoreAccessor) BeginRedelegate(
	_ context.Context,
	_, _ state.ValAddress,
	_, _ state.Int,
	_ uint64,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (c stubbedCoreAccessor) Undelegate(
	_ context.Context,
	_ state.ValAddress,
	_, _ state.Int,
	_ uint64,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (c stubbedCoreAccessor) Delegate(
	_ context.Context,
	_ state.ValAddress,
	_, _ state.Int,
	_ uint64,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (c stubbedCoreAccessor) QueryDelegation(
	context.Context,
	state.ValAddress,
) (*types.QueryDelegationResponse, error) {
	return nil, ErrNoStateAccess
}

func (c stubbedCoreAccessor) QueryUnbonding(
	context.Context,
	state.ValAddress,
) (*types.QueryUnbondingDelegationResponse, error) {
	return nil, ErrNoStateAccess
}

func (c stubbedCoreAccessor) QueryRedelegations(
	_ context.Context,
	_, _ state.ValAddress,
) (*types.QueryRedelegationsResponse, error) {
	return nil, ErrNoStateAccess
}
