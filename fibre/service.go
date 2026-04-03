package fibre

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

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

type Service struct {
	*AccountClient

	fibreClient *appfibre.Client
	txClient    *txclient.TxClient

	metrics *blobMetrics
}

func NewService(fiberClient *appfibre.Client, txClient *txclient.TxClient, accClient *AccountClient) *Service {
	c := &Service{
		fibreClient:   fiberClient,
		txClient:      txClient,
		AccountClient: accClient,
	}
	return c
}

func (s *Service) Submit(
	ctx context.Context,
	ns libshare.Namespace,
	data []byte,
	options *txclient.TxConfig,
) (_ *appfibre.PutResult, _ *appfibre.PaymentPromise, err error) {
	start := time.Now()
	defer func() {
		s.metrics.observeSubmit(ctx, time.Since(start), len(data), err)
	}()

	log.Infow("submitting blob", "namespace", ns.ID(), "data-size", len(data))

	blob, err := appfibre.NewBlob(data, appfibre.DefaultBlobConfigV0())
	if err != nil {
		return nil, nil, err
	}

	promise, err := s.upload(ctx, ns, blob)
	if err != nil {
		log.Errorw("uploading blob", "err", err, "namespace", ns.ID())
		return nil, nil, err
	}

	protoPromise, err := promise.ToProto()
	if err != nil {
		return nil, nil, err
	}

	signer, err := s.txClient.GetTxAuthorAccAddress(options)
	if err != nil {
		return nil, nil, fmt.Errorf("getting signer address: %w", err)
	}

	msg := &fibretypes.MsgPayForFibre{
		Signer:              signer.String(),
		PaymentPromise:      *protoPromise,
		ValidatorSignatures: promise.ValidatorSignatures,
	}
	resp, err := s.txClient.SubmitMessage(ctx, msg, options)
	if err != nil {
		log.Errorw("submitting blob", "err", err, "namespace", promise.Namespace)
		return nil, nil, err
	}

	putRes := &appfibre.PutResult{
		BlobID:              blob.ID(),
		ValidatorSignatures: promise.ValidatorSignatures,
		TxHash:              resp.TxHash,
		Height:              uint64(resp.Height),
	}

	// go does not allow slicing a function return value directly (e.g., f()[:]).
	commitment := putRes.BlobID.Commitment()
	log.Debugw("blob submitted",
		"namespace", promise.Namespace,
		"commitment", hex.EncodeToString(commitment[:]),
		"height", putRes.Height,
		"tx-hash", putRes.TxHash,
		"signatures", len(putRes.ValidatorSignatures),
	)
	return putRes, promise.PaymentPromise, nil
}

func (s *Service) Upload(
	ctx context.Context,
	ns libshare.Namespace,
	data []byte,
) (_ *appfibre.SignedPaymentPromise, err error) {
	start := time.Now()
	defer func() {
		s.metrics.observeUpload(ctx, time.Since(start), len(data), err)
	}()

	log.Infow("uploading blob", "namespace", ns.ID(), "data-size", len(data))

	blob, err := appfibre.NewBlob(data, appfibre.DefaultBlobConfigV0())
	if err != nil {
		return nil, err
	}
	promise, err := s.upload(ctx, ns, blob)
	if err != nil {
		log.Errorw("uploading blob", "err", err, "namespace", ns.ID())
		return nil, err
	}
	return promise, nil
}

func (s *Service) Get(ctx context.Context, ns libshare.Namespace, commitment []byte) (_ *appfibre.Blob, err error) {
	start := time.Now()
	defer func() {
		s.metrics.observeGet(ctx, time.Since(start), err)
	}()

	log.Debugw("getting blob", "namespace", ns.ID(), "commitment", hex.EncodeToString(commitment))
	if len(commitment) != appfibre.CommitmentSize {
		return nil, fmt.Errorf("commitment size does not match. want:%d got:%d", appfibre.CommitmentSize, len(commitment))
	}

	blobID := appfibre.NewBlobID(ns.Version(), appfibre.Commitment(commitment))
	return s.fibreClient.Download(ctx, blobID)
}

func (s *Service) upload(
	ctx context.Context,
	ns libshare.Namespace,
	blob *appfibre.Blob,
) (_ *appfibre.SignedPaymentPromise, err error) {
	promise, err := s.fibreClient.Upload(ctx, ns, blob)
	if err != nil {
		return nil, fmt.Errorf("failed to upload blob:%w", err)
	}

	commitment := promise.Commitment
	log.Debugw("blob uploaded",
		"namespace", ns.ID(),
		"commitment", hex.EncodeToString(commitment[:]),
		"signatures", len(promise.ValidatorSignatures),
	)
	return &promise, nil
}
