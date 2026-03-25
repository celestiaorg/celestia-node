package fibre

import (
	"context"

	sdktypes "github.com/cosmos/cosmos-sdk/types"

	appfibre "github.com/celestiaorg/celestia-app/v8/fibre"
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
	resp, pp, err := m.service.Submit(ctx, ns, data, options)
	if err != nil {
		return nil, err
	}

	blobID := appfibre.NewBlobID(uint8(pp.BlobVersion), pp.Commitment)
	uploadRes := UploadResult{
		BlobID:         blobID,
		PaymentPromise: toNodePaymentPromise(pp.PaymentPromise),
	}
	uploadRes.ValidatorSignatures = make([]ValidatorSignature, len(pp.ValidatorSignatures))
	for i, sig := range pp.ValidatorSignatures {
		uploadRes.ValidatorSignatures[i] = sig
	}

	return &SubmitResult{
		UploadResult: uploadRes,
		Height:       uint64(resp.Height),
		TxHash:       resp.TxHash,
	}, nil
}

func (m *module) Upload(
	ctx context.Context,
	ns libshare.Namespace,
	data []byte,
	options *txclient.TxConfig,
) (*UploadResult, error) {
	promise, blobID, err := m.service.Upload(ctx, ns, data, options)
	if err != nil {
		return nil, err
	}

	uploadRes := &UploadResult{
		BlobID:         blobID,
		PaymentPromise: toNodePaymentPromise(promise.PaymentPromise),
	}

	uploadRes.ValidatorSignatures = make([]ValidatorSignature, len(promise.ValidatorSignatures))
	for i, sig := range promise.ValidatorSignatures {
		uploadRes.ValidatorSignatures[i] = sig
	}
	return uploadRes, nil
}

func (m *module) Download(ctx context.Context, blobID appfibre.BlobID) (*GetBlobResult, error) {
	blob, err := m.service.Download(ctx, blobID)
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
