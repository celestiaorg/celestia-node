package rpc

import (
	"context"
	"errors"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/blob"
	blobapi "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	stateapi "github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/state"
)

var (
	ErrStateDisabled      = errors.New("state module is disabled")
	ErrBlobSubmitDisabled = errors.New("blob.Submit is disabled")
)

// disabledStateModule is a wrapper that disables all write operations of the state module
type disabledStateModule struct {
	stateapi.Module
}

func (s *disabledStateModule) Transfer(
	_ context.Context,
	_ state.AccAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrStateDisabled
}

func (s *disabledStateModule) SubmitPayForBlob(
	_ context.Context,
	_ []*libshare.Blob,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrStateDisabled
}

func (s *disabledStateModule) CancelUnbondingDelegation(
	_ context.Context,
	_ state.ValAddress,
	_, _ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrStateDisabled
}

func (s *disabledStateModule) BeginRedelegate(
	_ context.Context,
	_, _ state.ValAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrStateDisabled
}

func (s *disabledStateModule) Undelegate(
	_ context.Context,
	_ state.ValAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrStateDisabled
}

func (s *disabledStateModule) Delegate(
	_ context.Context,
	_ state.ValAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrStateDisabled
}

func (s *disabledStateModule) GrantFee(
	_ context.Context,
	_ state.AccAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrStateDisabled
}

func (s *disabledStateModule) RevokeGrantFee(
	_ context.Context,
	_ state.AccAddress,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrStateDisabled
}

// readOnlyBlobModule is a wrapper that disables the Submit operation of the blob module
type readOnlyBlobModule struct {
	blobapi.Module
}

func (b *readOnlyBlobModule) Submit(
	_ context.Context,
	_ []*blob.Blob,
	_ *blob.SubmitOptions,
) (uint64, error) {
	return 0, ErrBlobSubmitDisabled
}
