package fibre

import (
	"time"

	appfibre "github.com/celestiaorg/celestia-app/v8/fibre"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/fibre"
)

// ValidatorSignature is a signature from a validator.
type ValidatorSignature []byte

// UploadResult is the result of uploading a blob to FSPs without on-chain submission.
// It contains the blob commitment, aggregated validator signatures, and the payment promise
// that can be used for a later on-chain MsgPayForFibre submission.
type UploadResult struct {
	// Commitment is the Fibre commitment for the uploaded blob.
	Commitment fibre.Commitment `json:"commitment"`
	// ValidatorSignatures are attestations from validators confirming blob availability.
	ValidatorSignatures []ValidatorSignature `json:"validator_signatures"`
	// PaymentPromise is the signed promise used for on-chain fee settlement.
	PaymentPromise *PaymentPromise `json:"payment_promise,omitempty"`
	// RetentionUntil is the time until which FSPs will retain the blob data.
	RetentionUntil *time.Time `json:"retention_until,omitempty"`
}

// SubmitResult is the result of a full Fibre blob submission including on-chain MsgPayForFibre.
// It extends UploadResult with chain inclusion details (height and transaction hash).
type SubmitResult struct {
	// Commitment is the Fibre commitment for the submitted blob.
	Commitment fibre.Commitment `json:"commitment"`
	// ValidatorSignatures are attestations from validators confirming blob availability.
	ValidatorSignatures []ValidatorSignature `json:"validator_signatures"`
	// Height is the block height at which MsgPayForFibre was included on-chain.
	Height uint64 `json:"height"`
	// TxHash is the transaction hash of the on-chain MsgPayForFibre.
	TxHash string `json:"tx_hash"`
	// PaymentPromise is the signed promise used for on-chain fee settlement.
	PaymentPromise *PaymentPromise `json:"payment_promise,omitempty"`
	// RetentionUntil is the time until which FSPs will retain the blob data.
	RetentionUntil *time.Time `json:"retention_until,omitempty"`
}

// PaymentPromise represents the signed payment promise between a user and the Fibre network.
// It is constructed during upload and submitted on-chain as part of MsgPayForFibre.
type PaymentPromise struct {
	// ChainID is the chain identifier for which the promise is valid.
	ChainID string `json:"chain_id"`
	// Namespace is the blob namespace.
	Namespace libshare.Namespace `json:"namespace"`
	// BlobSize is the size of the uploaded blob data in bytes.
	BlobSize uint32 `json:"blob_size"`
	// Commitment is the Fibre commitment for the blob.
	Commitment fibre.Commitment `json:"commitment"`
	// RowVersion is the blob/share version used for encoding.
	RowVersion uint32 `json:"row_version"`
	// ValsetHeight is the validator set height at the time the promise was created.
	ValsetHeight uint64 `json:"valset_height"`
	// CreationTimestamp is the time when the payment promise was created.
	CreationTimestamp time.Time `json:"creation_timestamp"`
	// Signature is the cryptographic signature over the promise fields.
	Signature []byte `json:"signature"`
}

// GetBlobResult is the response from retrieving a Fibre blob.
type GetBlobResult struct {
	// Data is the raw blob data reconstructed from FSPs.
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
		Commitment:        fibre.Commitment(result.Commitment),
		RowVersion:        result.BlobVersion,
		ValsetHeight:      result.Height,
		CreationTimestamp: result.CreationTimestamp,
		Signature:         result.Signature,
	}
}
