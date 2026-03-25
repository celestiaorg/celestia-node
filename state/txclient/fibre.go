package txclient

import (
	"context"
	"errors"
	"fmt"

	appfibre "github.com/celestiaorg/celestia-app/v8/fibre"
	"github.com/celestiaorg/celestia-app/v8/x/fibre/types"
	libshare "github.com/celestiaorg/go-square/v4/share"
)

func (c *TxClient) Upload(
	ctx context.Context,
	ns libshare.Namespace,
	data []byte,
) (appfibre.SignedPaymentPromise, error) {
	if err := c.setupFibreClient(); err != nil {
		return appfibre.SignedPaymentPromise{}, err
	}
	blob, err := appfibre.NewBlob(data, appfibre.DefaultBlobConfigV0())
	if err != nil {
		return appfibre.SignedPaymentPromise{}, err
	}
	return c.fibreClient.Upload(ctx, ns, blob)
}

func (c *TxClient) Submit(
	ctx context.Context,
	ns libshare.Namespace,
	data []byte,
	cfg *TxConfig,
) (*appfibre.PutResult, *appfibre.PaymentPromise, error) {
	if err := c.setupFibreClient(); err != nil {
		return nil, nil, err
	}

	blob, err := appfibre.NewBlob(data, appfibre.DefaultBlobConfigV0())
	if err != nil {
		return nil, nil, err
	}

	promise, err := c.Upload(ctx, ns, data)
	if err != nil {
		return nil, nil, err
	}

	protoPromise, err := promise.ToProto()
	if err != nil {
		return nil, nil, err
	}

	signer, err := c.getTxAuthorAccAddress(cfg)
	if err != nil {
		return nil, nil, errors.New("can't get signer address")
	}

	msg := &types.MsgPayForFibre{
		Signer:              signer.String(),
		PaymentPromise:      *protoPromise,
		ValidatorSignatures: promise.ValidatorSignatures,
	}

	resp, err := c.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return nil, nil, err
	}

	putRes := &appfibre.PutResult{
		BlobID:              blob.ID(),
		ValidatorSignatures: promise.ValidatorSignatures,
		TxHash:              resp.TxHash,
		Height:              uint64(resp.Height),
	}
	return putRes, promise.PaymentPromise, nil
}

func (c *TxClient) Get(ctx context.Context, ns libshare.Namespace, commitment []byte) (*appfibre.Blob, error) {
	if err := c.setupFibreClient(); err != nil {
		return nil, err
	}
	if len(commitment) != appfibre.CommitmentSize {
		return nil, fmt.Errorf("commitment size does not match. want:%d got:%d", appfibre.CommitmentSize, len(commitment))
	}
	blobID := appfibre.NewBlobID(ns.Version(), appfibre.Commitment(commitment))
	return c.fibreClient.Download(ctx, blobID)
}

func (c *TxClient) setupFibreClient() error {
	c.fibreClientLk.Lock()
	defer c.fibreClientLk.Unlock()

	if c.fibreClient != nil {
		return nil
	}

	cl, err := appfibre.NewClient(c.keyring, appfibre.DefaultClientConfig())
	if err != nil {
		return fmt.Errorf("failed to setup fibre client: %w", err)
	}
	if err := cl.Start(c.ctx); err != nil {
		return fmt.Errorf("failed to start fibre client: %w", err)
	}
	c.fibreClient = cl
	return nil
}
