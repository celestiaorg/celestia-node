package state

import (
	"context"

	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/staking/types"

	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/state"
)

var _ Module = (*API)(nil)

// Module represents the behaviors necessary for a user to
// query for state-related information and submit transactions/
// messages to the Celestia network.
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
//nolint:dupl
type Module interface {
	// AccountAddress retrieves the address of the node's account/signer
	AccountAddress(ctx context.Context) (state.Address, error)
	// Balance retrieves the Celestia coin balance for the node's account/signer
	// and verifies it against the corresponding block's AppHash.
	Balance(ctx context.Context) (*state.Balance, error)
	// BalanceForAddress retrieves the Celestia coin balance for the given address and verifies
	// the returned balance against the corresponding block's AppHash.
	//
	// NOTE: the balance returned is the balance reported by the block right before
	// the node's current head (head-1). This is due to the fact that for block N, the block's
	// `AppHash` is the result of applying the previous block's transaction list.
	BalanceForAddress(ctx context.Context, addr state.Address) (*state.Balance, error)
	// Transfer sends the given amount of coins from default wallet of the node to the given account
	// address.
	Transfer(
		ctx context.Context, to state.AccAddress, amount state.Int, config *state.TxConfig,
	) (*state.TxResponse, error)
	// SubmitPayForBlob builds, signs and submits a PayForBlob transaction.
	SubmitPayForBlob(
		ctx context.Context,
		blobs []*libshare.Blob,
		config *state.TxConfig,
	) (*state.TxResponse, error)
	// CancelUnbondingDelegation cancels a user's pending undelegation from a validator.
	CancelUnbondingDelegation(
		ctx context.Context,
		valAddr state.ValAddress,
		amount,
		height state.Int,
		config *state.TxConfig,
	) (*state.TxResponse, error)
	// BeginRedelegate sends a user's delegated tokens to a new validator for redelegation.
	BeginRedelegate(
		ctx context.Context,
		srcValAddr,
		dstValAddr state.ValAddress,
		amount state.Int,
		config *state.TxConfig,
	) (*state.TxResponse, error)
	// Undelegate undelegates a user's delegated tokens, unbonding them from the current validator.
	Undelegate(
		ctx context.Context,
		delAddr state.ValAddress,
		amount state.Int,
		config *state.TxConfig,
	) (*state.TxResponse, error)
	// Delegate sends a user's liquid tokens to a validator for delegation.
	Delegate(
		ctx context.Context,
		delAddr state.ValAddress,
		amount state.Int,
		config *state.TxConfig,
	) (*state.TxResponse, error)
	// WithdrawDelegatorReward withdraws a delegator's rewards from a validator.
	WithdrawDelegatorReward(
		ctx context.Context,
		valAddr state.ValAddress,
		config *state.TxConfig,
	) (*state.TxResponse, error)

	// QueryDelegationRewards retrieves the pending rewards for a delegation to a given validator.
	QueryDelegationRewards(
		ctx context.Context,
		valAddr state.ValAddress,
	) (*distributiontypes.QueryDelegationRewardsResponse, error)
	// QueryDelegation retrieves the delegation information between a delegator and a validator.
	QueryDelegation(ctx context.Context, valAddr state.ValAddress) (*types.QueryDelegationResponse, error)
	// QueryUnbonding retrieves the unbonding status between a delegator and a validator.
	QueryUnbonding(ctx context.Context, valAddr state.ValAddress) (*types.QueryUnbondingDelegationResponse, error)
	// QueryRedelegations retrieves the status of the redelegations between a delegator and a validator.
	QueryRedelegations(
		ctx context.Context,
		srcValAddr,
		dstValAddr state.ValAddress,
	) (*types.QueryRedelegationsResponse, error)

	GrantFee(
		ctx context.Context,
		grantee state.AccAddress,
		amount state.Int,
		config *state.TxConfig,
	) (*state.TxResponse, error)

	RevokeGrantFee(
		ctx context.Context,
		grantee state.AccAddress,
		config *state.TxConfig,
	) (*state.TxResponse, error)
}

// API is a wrapper around Module for the RPC.
//
//nolint:dupl
type API struct {
	Internal struct {
		AccountAddress    func(ctx context.Context) (state.Address, error)                      `perm:"read"`
		Balance           func(ctx context.Context) (*state.Balance, error)                     `perm:"read"`
		BalanceForAddress func(ctx context.Context, addr state.Address) (*state.Balance, error) `perm:"read"`
		Transfer          func(
			ctx context.Context,
			to state.AccAddress,
			amount state.Int,
			config *state.TxConfig,
		) (*state.TxResponse, error) `perm:"write"`
		SubmitPayForBlob func(
			ctx context.Context,
			blobs []*libshare.Blob,
			config *state.TxConfig,
		) (*state.TxResponse, error) `perm:"write"`
		CancelUnbondingDelegation func(
			ctx context.Context,
			valAddr state.ValAddress,
			amount, height state.Int,
			config *state.TxConfig,
		) (*state.TxResponse, error) `perm:"write"`
		BeginRedelegate func(
			ctx context.Context,
			srcValAddr,
			dstValAddr state.ValAddress,
			amount state.Int,
			config *state.TxConfig,
		) (*state.TxResponse, error) `perm:"write"`
		Undelegate func(
			ctx context.Context,
			delAddr state.ValAddress,
			amount state.Int,
			config *state.TxConfig,
		) (*state.TxResponse, error) `perm:"write"`
		Delegate func(
			ctx context.Context,
			delAddr state.ValAddress,
			amount state.Int,
			config *state.TxConfig,
		) (*state.TxResponse, error) `perm:"write"`
		WithdrawDelegatorReward func(
			ctx context.Context,
			valAddr state.ValAddress,
			config *state.TxConfig,
		) (*state.TxResponse, error) `perm:"write"`
		QueryDelegationRewards func(
			ctx context.Context,
			valAddr state.ValAddress,
		) (*distributiontypes.QueryDelegationRewardsResponse, error) `perm:"read"`
		QueryDelegation func(
			ctx context.Context,
			valAddr state.ValAddress,
		) (*types.QueryDelegationResponse, error) `perm:"read"`
		QueryUnbonding func(
			ctx context.Context,
			valAddr state.ValAddress,
		) (*types.QueryUnbondingDelegationResponse, error) `perm:"read"`
		QueryRedelegations func(
			ctx context.Context,
			srcValAddr,
			dstValAddr state.ValAddress,
		) (*types.QueryRedelegationsResponse, error) `perm:"read"`
		GrantFee func(
			ctx context.Context,
			grantee state.AccAddress,
			amount state.Int,
			config *state.TxConfig,
		) (*state.TxResponse, error) `perm:"write"`
		RevokeGrantFee func(
			ctx context.Context,
			grantee state.AccAddress,
			config *state.TxConfig,
		) (*state.TxResponse, error) `perm:"write"`
	}
}

func (api *API) AccountAddress(ctx context.Context) (state.Address, error) {
	return api.Internal.AccountAddress(ctx)
}

func (api *API) BalanceForAddress(ctx context.Context, addr state.Address) (*state.Balance, error) {
	return api.Internal.BalanceForAddress(ctx, addr)
}

func (api *API) Transfer(
	ctx context.Context,
	to state.AccAddress,
	amount state.Int,
	config *state.TxConfig,
) (*state.TxResponse, error) {
	return api.Internal.Transfer(ctx, to, amount, config)
}

func (api *API) SubmitPayForBlob(
	ctx context.Context,
	blobs []*libshare.Blob,
	config *state.TxConfig,
) (*state.TxResponse, error) {
	return api.Internal.SubmitPayForBlob(ctx, blobs, config)
}

func (api *API) CancelUnbondingDelegation(
	ctx context.Context,
	valAddr state.ValAddress,
	amount, height state.Int,
	config *state.TxConfig,
) (*state.TxResponse, error) {
	return api.Internal.CancelUnbondingDelegation(ctx, valAddr, amount, height, config)
}

func (api *API) BeginRedelegate(
	ctx context.Context,
	srcValAddr, dstValAddr state.ValAddress,
	amount state.Int,
	config *state.TxConfig,
) (*state.TxResponse, error) {
	return api.Internal.BeginRedelegate(ctx, srcValAddr, dstValAddr, amount, config)
}

func (api *API) Undelegate(
	ctx context.Context,
	delAddr state.ValAddress,
	amount state.Int,
	config *state.TxConfig,
) (*state.TxResponse, error) {
	return api.Internal.Undelegate(ctx, delAddr, amount, config)
}

func (api *API) Delegate(
	ctx context.Context,
	delAddr state.ValAddress,
	amount state.Int,
	config *state.TxConfig,
) (*state.TxResponse, error) {
	return api.Internal.Delegate(ctx, delAddr, amount, config)
}

func (api *API) WithdrawDelegatorReward(
	ctx context.Context,
	valAddr state.ValAddress,
	config *state.TxConfig,
) (*state.TxResponse, error) {
	return api.Internal.WithdrawDelegatorReward(ctx, valAddr, config)
}

func (api *API) QueryDelegationRewards(
	ctx context.Context,
	valAddr state.ValAddress,
) (*distributiontypes.QueryDelegationRewardsResponse, error) {
	return api.Internal.QueryDelegationRewards(ctx, valAddr)
}

func (api *API) QueryDelegation(ctx context.Context, valAddr state.ValAddress) (*types.QueryDelegationResponse, error) {
	return api.Internal.QueryDelegation(ctx, valAddr)
}

func (api *API) QueryUnbonding(
	ctx context.Context,
	valAddr state.ValAddress,
) (*types.QueryUnbondingDelegationResponse, error) {
	return api.Internal.QueryUnbonding(ctx, valAddr)
}

func (api *API) QueryRedelegations(
	ctx context.Context,
	srcValAddr, dstValAddr state.ValAddress,
) (*types.QueryRedelegationsResponse, error) {
	return api.Internal.QueryRedelegations(ctx, srcValAddr, dstValAddr)
}

func (api *API) Balance(ctx context.Context) (*state.Balance, error) {
	return api.Internal.Balance(ctx)
}

func (api *API) GrantFee(
	ctx context.Context,
	grantee state.AccAddress,
	amount state.Int,
	config *state.TxConfig,
) (*state.TxResponse, error) {
	return api.Internal.GrantFee(ctx, grantee, amount, config)
}

func (api *API) RevokeGrantFee(
	ctx context.Context,
	grantee state.AccAddress,
	config *state.TxConfig,
) (*state.TxResponse, error) {
	return api.Internal.RevokeGrantFee(ctx, grantee, config)
}
