package fibre

import (
	"context"

	sdktypes "github.com/cosmos/cosmos-sdk/types"

	appfibre "github.com/celestiaorg/celestia-app/v8/fibre"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/fibre"
	"github.com/celestiaorg/celestia-node/state/txclient"
)

var _ Module = (*API)(nil)

// Module defines the API related to interacting with the Fibre network.
// Fibre enables off-chain blob storage via Fibre Storage Providers (FSPs) with on-chain
// payment settlement. Full blob submission (upload + on-chain MsgPayForFibre) is available
// through blob.SubmitFibreBlob; this module exposes upload-only, retrieval, and escrow operations.
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// Submit submits a v0 Fibre blob via the Fibre network.
	// It performs the full Fibre flow: uploads blob data to FSPs, aggregates validator
	// availability signatures, and submits MsgPayForFibre on-chain.
	// Returns the submission result including the on-chain height and transaction hash.
	// Requires the node to be connected to a core endpoint with Fibre support.
	// NOTE: Currently only v0 Fibre blobs are supported.
	Submit(context.Context, libshare.Namespace, []byte, *txclient.TxConfig) (*SubmitResult, error)
	// Upload performs the off-chain portion of v0 Fibre blob submission only.
	// It encodes the blob, constructs a payment promise, uploads encoded rows to FSPs,
	// and aggregates validator availability signatures. It does NOT submit MsgPayForFibre on-chain.
	// Use fibre.Submit for the full submit flow.
	// NOTE: Currently only v0 Fibre blobs are supported.
	Upload(context.Context, libshare.Namespace, []byte, *txclient.TxConfig) (*UploadResult, error)
	// Download retrieves a Fibre blob from FSPs by blobID.
	// It reconstructs the original blob data from the encoded rows stored off-chain.
	Download(context.Context, appfibre.BlobID) (*GetBlobResult, error)
	// QueryEscrowAccount returns the escrow account details for the given signer address,
	// including total balance and available (spendable) balance.
	QueryEscrowAccount(_ context.Context, signer string) (*fibre.EscrowAccount, error)
	// Deposit adds funds to the node's Fibre escrow account.
	// The signer is resolved from cfg (SignerAddress or KeyName) or the node's default account.
	Deposit(context.Context, sdktypes.Coin, *txclient.TxConfig) error
	// Withdraw requests a withdrawal from the node's Fibre escrow account.
	// The signer is resolved from cfg (SignerAddress or KeyName) or the node's default account.
	// The withdrawal enters an unbonding period before funds become claimable.
	Withdraw(context.Context, sdktypes.Coin, *txclient.TxConfig) error
	// PendingWithdrawals returns all pending (not yet claimable) withdrawals for the given signer.
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
