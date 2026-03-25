package fibre

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	logging "github.com/ipfs/go-log/v2"

	appfibre "github.com/celestiaorg/celestia-app/v8/fibre"
	fibretypes "github.com/celestiaorg/celestia-app/v8/x/fibre/types"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/state/txclient"
)

var (
	log = logging.Logger("fibre")

	// ErrClientNotAvailable is returned when fibre.Client methods are called on a nil receiver.
	ErrClientNotAvailable = errors.New("fibre client is not available: node is not connected to a core endpoint")
)

// client defines the interface for interacting with the transaction client.
type client interface {
	Upload(ctx context.Context, ns libshare.Namespace, data []byte) (appfibre.SignedPaymentPromise, error)
	Submit(
		ctx context.Context, ns libshare.Namespace, data []byte, options *txclient.TxConfig,
	) (*appfibre.PutResult, *appfibre.PaymentPromise, error)
	Get(ctx context.Context, ns libshare.Namespace, commitment []byte) (*appfibre.Blob, error)
	QueryEscrowAccount(ctx context.Context, signer string) (*fibretypes.EscrowAccount, error)
	Deposit(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error
	Withdraw(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error
	PendingWithdrawals(ctx context.Context, signer string) ([]fibretypes.Withdrawal, error)
}

type Client struct {
	client  client
	account *AccountClient

	metrics *blobMetrics
}

func NewClient(client client) *Client {
	c := &Client{client: client}
	c.account = &AccountClient{client: client}
	return c
}

func (c *Client) Upload(ctx context.Context, ns Namespace, data []byte) (_ *UploadResult, err error) {
	if c == nil {
		return nil, ErrClientNotAvailable
	}

	start := time.Now()
	defer func() {
		c.metrics.observeUpload(ctx, time.Since(start), len(data), err)
	}()

	log.Infow("uploading blob", "namespace", ns.ID(), "data-size", len(data))
	promise, err := c.client.Upload(ctx, ns, data)
	if err != nil {
		log.Errorw("uploading blob", "err", err, "namespace", ns.ID())
		return nil, err
	}

	uploadRes := &UploadResult{
		Commitment:     Commitment(promise.Commitment),
		PaymentPromise: getPaymentPromise(promise.PaymentPromise),
	}

	uploadRes.ValidatorSignatures = make([]ValidatorSignature, len(promise.ValidatorSignatures))
	for i, sig := range promise.ValidatorSignatures {
		uploadRes.ValidatorSignatures[i] = sig
	}

	log.Debugw("blob uploaded",
		"namespace", ns.ID(),
		"commitment", hex.EncodeToString(uploadRes.Commitment[:]),
		"signatures", len(uploadRes.ValidatorSignatures),
	)
	return uploadRes, nil
}

func (c *Client) Get(ctx context.Context, ns Namespace, commitment []byte) (_ *GetBlobResponse, err error) {
	if c == nil {
		return nil, ErrClientNotAvailable
	}

	start := time.Now()
	defer func() {
		c.metrics.observeGet(ctx, time.Since(start), err)
	}()

	log.Debugw("getting blob", "namespace", ns.ID(), "commitment", hex.EncodeToString(commitment))
	blob, err := c.client.Get(ctx, ns, commitment)
	if err != nil {
		log.Errorw("getting blob", "err", err, "namespace", ns.ID())
		return nil, err
	}

	log.Debugw("blob retrieved", "namespace", ns.ID(), "data-size", len(blob.Data()))
	return &GetBlobResponse{
		Data: blob.Data(),
	}, nil
}

func (c *Client) Submit(
	ctx context.Context,
	ns Namespace,
	data []byte,
	options *txclient.TxConfig,
) (_ *SubmitResult, err error) {
	if c == nil {
		return nil, ErrClientNotAvailable
	}

	start := time.Now()
	defer func() {
		c.metrics.observeSubmit(ctx, time.Since(start), len(data), err)
	}()

	log.Infow("submitting blob", "namespace", ns.ID(), "data-size", len(data))
	putRes, promise, err := c.client.Submit(ctx, ns, data, options)
	if err != nil {
		log.Errorw("submitting blob", "err", err, "namespace", ns.ID())
		return nil, err
	}

	submitRes := &SubmitResult{
		Commitment:     Commitment(putRes.BlobID.Commitment()),
		Height:         putRes.Height,
		TxHash:         putRes.TxHash,
		PaymentPromise: getPaymentPromise(promise),
	}

	submitRes.ValidatorSignatures = make([]ValidatorSignature, len(putRes.ValidatorSignatures))
	for i, sig := range putRes.ValidatorSignatures {
		submitRes.ValidatorSignatures[i] = sig
	}

	log.Debugw("blob submitted",
		"namespace", ns.ID(),
		"commitment", hex.EncodeToString(submitRes.Commitment[:]),
		"height", submitRes.Height,
		"tx-hash", submitRes.TxHash,
		"signatures", len(submitRes.ValidatorSignatures),
	)
	return submitRes, nil
}

func (c *Client) Account() *AccountClient {
	return c.account
}

func getPaymentPromise(result *appfibre.PaymentPromise) *PaymentPromise {
	if result == nil {
		return nil
	}
	return &PaymentPromise{
		ChainID:           result.ChainID,
		Namespace:         result.Namespace,
		BlobSize:          result.UploadSize,
		Commitment:        Commitment(result.Commitment),
		RowVersion:        result.BlobVersion,
		ValsetHeight:      result.Height,
		CreationTimestamp: result.CreationTimestamp,
		Signature:         result.Signature,
	}
}
