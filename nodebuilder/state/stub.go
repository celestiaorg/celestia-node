package state

import (
	"context"
	"errors"

	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/staking/types"

	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/state"
)

var ErrNoStateAccess = errors.New("node is running without state access. run with --core.ip <CORE NODE IP> to resolve")

// stubbedStateModule provides a stub for the state module to return
// errors when state endpoints are accessed without a running connection
// to a core endpoint.
type stubbedStateModule struct{}

func (s stubbedStateModule) AccountAddress(context.Context) (state.Address, error) {
	return state.Address{}, ErrNoStateAccess
}

func (s stubbedStateModule) Balance(context.Context) (*state.Balance, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) BalanceForAddress(
	context.Context,
	state.Address,
) (*state.Balance, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) Transfer(
	_ context.Context,
	_ state.AccAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) SubmitPayForBlob(
	context.Context,
	[]*libshare.Blob,
	*state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) CancelUnbondingDelegation(
	_ context.Context,
	_ state.ValAddress,
	_, _ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) BeginRedelegate(
	_ context.Context,
	_, _ state.ValAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) Undelegate(
	_ context.Context,
	_ state.ValAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) Delegate(
	_ context.Context,
	_ state.ValAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) WithdrawDelegatorReward(
	_ context.Context,
	_ state.ValAddress,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) QueryDelegationRewards(
	context.Context,
	state.ValAddress,
) (*distributiontypes.QueryDelegationRewardsResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) QueryDelegation(
	context.Context,
	state.ValAddress,
) (*types.QueryDelegationResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) QueryUnbonding(
	context.Context,
	state.ValAddress,
) (*types.QueryUnbondingDelegationResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) QueryRedelegations(
	_ context.Context,
	_, _ state.ValAddress,
) (*types.QueryRedelegationsResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) GrantFee(
	_ context.Context,
	_ state.AccAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}

func (s stubbedStateModule) RevokeGrantFee(
	_ context.Context,
	_ state.AccAddress,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrNoStateAccess
}
