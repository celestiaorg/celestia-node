package client

import (
	"context"
	"errors"

	"github.com/cosmos/cosmos-sdk/x/staking/types"

	libhead "github.com/celestiaorg/go-header"
	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	blobapi "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	headerapi "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/state"
)

var _ blobapi.Module = (*blobSubmitClient)(nil)

var ErrReadOnlyMode = errors.New("submit is disabled in read only client")

type readOnlyBlobAPI struct {
	blobapi.Module
}

type blobSubmitClient struct {
	blobapi.Module
	submitter *blob.Service
}

func (api *readOnlyBlobAPI) Submit(context.Context, []*blob.Blob, *blob.SubmitOptions) (uint64, error) {
	return 0, ErrReadOnlyMode
}

func (api *blobSubmitClient) Submit(ctx context.Context,
	blobs []*blob.Blob, options *blob.SubmitOptions,
) (uint64, error) {
	if api.submitter == nil {
		return 0, errors.New("key needs to be set before blob.Submit can be used")
	}
	// TODO: this is a hack to allow nil options, because it's not exported by the blob package
	if options == nil {
		options = &blob.SubmitOptions{}
	}
	return api.submitter.Submit(ctx, blobs, options)
}

// readOnlyStateAPI returns an error on all state client methods if consensus connection is not
// configured
type readOnlyStateAPI struct{}

func (r *readOnlyStateAPI) AccountAddress(context.Context) (state.Address, error) {
	return state.Address{}, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) Balance(context.Context) (*state.Balance, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) BalanceForAddress(context.Context,
	state.Address,
) (*state.Balance, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) Transfer(context.Context,
	state.AccAddress, state.Int, *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) SubmitPayForBlob(context.Context,
	[]*libshare.Blob, *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) CancelUnbondingDelegation(context.Context,
	state.ValAddress, state.Int, state.Int, *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) BeginRedelegate(context.Context,
	state.ValAddress, state.ValAddress, state.Int, *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) Undelegate(context.Context,
	state.ValAddress, state.Int, *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) Delegate(context.Context,
	state.ValAddress, state.Int, *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) QueryDelegation(context.Context,
	state.ValAddress,
) (*types.QueryDelegationResponse, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) QueryUnbonding(context.Context,
	state.ValAddress,
) (*types.QueryUnbondingDelegationResponse, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) QueryRedelegations(context.Context,
	state.ValAddress, state.ValAddress,
) (*types.QueryRedelegationsResponse, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) GrantFee(context.Context,
	state.AccAddress, state.Int, *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (r *readOnlyStateAPI) RevokeGrantFee(context.Context,
	state.AccAddress, *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

type trustedHeadGetter struct {
	remote headerapi.Module
}

func (t trustedHeadGetter) Head(
	ctx context.Context,
	_ ...libhead.HeadOption[*header.ExtendedHeader],
) (*header.ExtendedHeader, error) {
	return t.remote.NetworkHead(ctx)
}
