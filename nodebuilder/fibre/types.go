package fibre

import (
	"time"

	appfibre "github.com/celestiaorg/celestia-app/v10/fibre"
	libshare "github.com/celestiaorg/go-square/v4/share"
)

// ValidatorSignature is a validator's blob-availability attestation.
type ValidatorSignature []byte

// UploadResult is returned by an off-chain Fibre upload; PaymentPromise can later
// settle it on-chain via MsgPayForFibre.
type UploadResult struct {
	BlobID              appfibre.BlobID      `json:"blob_id"`
	ValidatorSignatures []ValidatorSignature `json:"validator_signatures"`
	PaymentPromise      *PaymentPromise      `json:"payment_promise,omitempty"`
}

// SubmitResult is an UploadResult plus the on-chain inclusion details of its settlement.
type SubmitResult struct {
	UploadResult
	Height uint64 `json:"height"`
	TxHash string `json:"tx_hash"`
}

// PaymentPromise is the signed user↔network promise, settled on-chain via MsgPayForFibre.
type PaymentPromise struct {
	ChainID           string              `json:"chain_id"`
	Namespace         libshare.Namespace  `json:"namespace"`
	BlobSize          uint32              `json:"blob_size"`
	Commitment        appfibre.Commitment `json:"commitment"`
	RowVersion        uint32              `json:"row_version"`
	ValsetHeight      uint64              `json:"valset_height"`
	CreationTimestamp time.Time           `json:"creation_timestamp"`
	Signature         []byte              `json:"signature"`
}

// GetBlobResult holds a blob reconstructed from FSPs.
type GetBlobResult struct {
	Data []byte `json:"data"`
}

func toNodePaymentPromise(result *appfibre.PaymentPromise) *PaymentPromise {
	if result == nil {
		return nil
	}
	return &PaymentPromise{
		ChainID:           result.ChainID,
		Namespace:         result.Namespace,
		BlobSize:          result.UploadSize,
		Commitment:        result.Commitment,
		RowVersion:        result.BlobVersion,
		ValsetHeight:      result.Height,
		CreationTimestamp: result.CreationTimestamp,
		Signature:         result.Signature,
	}
}
