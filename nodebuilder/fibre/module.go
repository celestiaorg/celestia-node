package fibre

import (
	"context"

	sdktypes "github.com/cosmos/cosmos-sdk/types"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/fibre"
	"github.com/celestiaorg/celestia-node/state/txclient"
)

// module wraps the fibre Client and AccountClient to implement Module.
type module struct {
	service *fibre.Service
}

func newModule(service *fibre.Service) *module {
	return &module{
		service: service,
	}
}

func (m *module) Submit(
	ctx context.Context,
	ns libshare.Namespace,
	data []byte,
	options *txclient.TxConfig,
) (*SubmitResult, error) {
	putRes, pp, err := m.service.Submit(ctx, ns, data, options)
	if err != nil {
		return nil, err
	}

	submitRes := &SubmitResult{
		Commitment:     putRes.BlobID.Commitment(),
		Height:         putRes.Height,
		TxHash:         putRes.TxHash,
		PaymentPromise: toNodePaymentPromise(pp),
	}

	submitRes.ValidatorSignatures = make([]ValidatorSignature, len(putRes.ValidatorSignatures))
	for i, sig := range putRes.ValidatorSignatures {
		submitRes.ValidatorSignatures[i] = sig
	}
	return submitRes, nil
}

func (m *module) Upload(
	ctx context.Context,
	ns libshare.Namespace,
	data []byte,
	options *txclient.TxConfig,
) (*UploadResult, error) {
	promise, err := m.service.Upload(ctx, ns, data, options)
	if err != nil {
		return nil, err
	}

	uploadRes := &UploadResult{
		Commitment:     promise.Commitment,
		PaymentPromise: toNodePaymentPromise(promise.PaymentPromise),
	}

	uploadRes.ValidatorSignatures = make([]ValidatorSignature, len(promise.ValidatorSignatures))
	for i, sig := range promise.ValidatorSignatures {
		uploadRes.ValidatorSignatures[i] = sig
	}
	return uploadRes, nil
}

func (m *module) Get(ctx context.Context, ns libshare.Namespace, commitment []byte) (*GetBlobResult, error) {
	blob, err := m.service.Get(ctx, ns, commitment)
	if err != nil {
		return nil, err
	}
	return &GetBlobResult{
		Data: blob.Data(),
	}, nil
}

func (m *module) QueryEscrowAccount(ctx context.Context, signer string) (*fibre.EscrowAccount, error) {
	return m.service.QueryEscrowAccount(ctx, signer)
}

func (m *module) Deposit(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error {
	return m.service.Deposit(ctx, amount, cfg)
}

func (m *module) Withdraw(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error {
	return m.service.Withdraw(ctx, amount, cfg)
}

func (m *module) PendingWithdrawals(ctx context.Context, signer string) ([]fibre.PendingWithdrawal, error) {
	return m.service.PendingWithdrawals(ctx, signer)
}
