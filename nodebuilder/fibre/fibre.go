package fibre

import (
	"context"

	sdktypes "github.com/cosmos/cosmos-sdk/types"

	appfibre "github.com/celestiaorg/celestia-app/v10/fibre"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/fibre"
	"github.com/celestiaorg/celestia-node/state/txclient"
)

var _ Module = (*API)(nil)

// Module is the RPC API for the Fibre network: off-chain blob storage via Fibre
// Storage Providers (FSPs) with on-chain payment settlement. Only v0 blobs are
// supported for now. The signer for escrow/settlement operations is resolved from
// the TxConfig (SignerAddress or KeyName), falling back to the node's default account.
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// Submit runs the full flow: upload to FSPs, aggregate validator signatures,
	// and settle MsgPayForFibre on-chain. Requires a Fibre-capable core endpoint.
	Submit(context.Context, libshare.Namespace, []byte, *txclient.TxConfig) (*SubmitResult, error)
	// Upload does the off-chain half only (encode, promise, upload, aggregate
	// signatures) and does NOT settle on-chain. Use Submit for the full flow.
	Upload(context.Context, libshare.Namespace, []byte, *txclient.TxConfig) (*UploadResult, error)
	// Download reconstructs a blob from FSPs by blobID.
	Download(context.Context, appfibre.BlobID) (*GetBlobResult, error)
	// QueryEscrowAccount returns the escrow account for the given signer.
	QueryEscrowAccount(_ context.Context, signer string) (*fibre.EscrowAccount, error)
	// Deposit adds funds to the node's escrow account.
	Deposit(context.Context, sdktypes.Coin, *txclient.TxConfig) error
	// Withdraw requests a withdrawal; funds unbond before becoming claimable.
	Withdraw(context.Context, sdktypes.Coin, *txclient.TxConfig) error
	// PendingWithdrawals returns not-yet-claimable withdrawals for the signer.
	PendingWithdrawals(_ context.Context, signer string) ([]fibre.PendingWithdrawal, error)
}

// API is a wrapper around Module for the RPC.
type API struct {
	Internal struct {
		Submit func(
			context.Context,
			libshare.Namespace,
			[]byte,
			*txclient.TxConfig,
		) (*SubmitResult, error) `perm:"write"`
		Upload func(
			ctx context.Context,
			ns libshare.Namespace,
			data []byte,
			config *txclient.TxConfig,
		) (*UploadResult, error) `perm:"write"`
		Download func(
			ctx context.Context,
			blobID appfibre.BlobID,
		) (*GetBlobResult, error) `perm:"read"`
		QueryEscrowAccount func(
			ctx context.Context,
			signer string,
		) (*fibre.EscrowAccount, error) `perm:"read"`
		Deposit func(
			ctx context.Context,
			amount sdktypes.Coin,
			cfg *txclient.TxConfig,
		) error `perm:"write"`
		Withdraw func(
			ctx context.Context,
			amount sdktypes.Coin,
			cfg *txclient.TxConfig,
		) error `perm:"write"`
		PendingWithdrawals func(
			ctx context.Context,
			signer string,
		) ([]fibre.PendingWithdrawal, error) `perm:"read"`
	}
}

func (api *API) Submit(
	ctx context.Context,
	ns libshare.Namespace,
	data []byte,
	options *txclient.TxConfig,
) (*SubmitResult, error) {
	return api.Internal.Submit(ctx, ns, data, options)
}

func (api *API) Upload(
	ctx context.Context,
	ns libshare.Namespace,
	data []byte,
	options *txclient.TxConfig,
) (*UploadResult, error) {
	return api.Internal.Upload(ctx, ns, data, options)
}

func (api *API) Download(ctx context.Context, blobID appfibre.BlobID) (*GetBlobResult, error) {
	return api.Internal.Download(ctx, blobID)
}

func (api *API) QueryEscrowAccount(ctx context.Context, signer string) (*fibre.EscrowAccount, error) {
	return api.Internal.QueryEscrowAccount(ctx, signer)
}

func (api *API) Deposit(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error {
	return api.Internal.Deposit(ctx, amount, cfg)
}

func (api *API) Withdraw(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error {
	return api.Internal.Withdraw(ctx, amount, cfg)
}

func (api *API) PendingWithdrawals(ctx context.Context, signer string) ([]fibre.PendingWithdrawal, error) {
	return api.Internal.PendingWithdrawals(ctx, signer)
}
