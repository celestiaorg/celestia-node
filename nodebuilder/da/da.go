package da

import (
	"context"

	"github.com/rollkit/go-da"
)

var _ da.DA = (Module)(nil)

// Deprecated: The DA API is experimental and deprecated. It is no longer supported and will be removed in the future.
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// MaxBlobSize returns the max blob size
	//
	// Deprecated: This method is deprecated and will be removed in the future.
	MaxBlobSize(ctx context.Context) (uint64, error)

	// Get returns Blob for each given ID, or an error.
	//
	// Error should be returned if ID is not formatted properly, there is no Blob for given ID or any other client-level
	// error occurred (dropped connection, timeout, etc).
	//
	// Deprecated: This method is deprecated and will be removed in the future.
	Get(ctx context.Context, ids []da.ID, namespace da.Namespace) ([]da.Blob, error)

	// GetIDs returns IDs of all Blobs located in DA at given height.
	//
	// Deprecated: This method is deprecated and will be removed in the future.
	GetIDs(ctx context.Context, height uint64, namespace da.Namespace) (*da.GetIDsResult, error)

	// GetProofs returns inclusion Proofs for Blobs specified by their IDs.
	//
	// Deprecated: This method is deprecated and will be removed in the future.
	GetProofs(ctx context.Context, ids []da.ID, namespace da.Namespace) ([]da.Proof, error)

	// Commit creates a Commitment for each given Blob.
	//
	// Deprecated: This method is deprecated and will be removed in the future.
	Commit(ctx context.Context, blobs []da.Blob, namespace da.Namespace) ([]da.Commitment, error)

	// Submit submits the Blobs to Data Availability layer.
	//
	// This method is synchronous. Upon successful submission to Data Availability layer, it returns the IDs identifying
	// blobs in DA.
	//
	// Deprecated: This method is deprecated and will be removed in the future.
	Submit(ctx context.Context, blobs []da.Blob, gasPrice float64, namespace da.Namespace) ([]da.ID, error)

	// SubmitWithOptions submits the Blobs to Data Availability layer.
	//
	// This method is synchronous. Upon successful submission to Data Availability layer, it returns the IDs identifying
	// blobs in DA.
	//
	// Deprecated: This method is deprecated and will be removed in the future.
	SubmitWithOptions(
		ctx context.Context, blobs []da.Blob, gasPrice float64, namespace da.Namespace, options []byte) ([]da.ID, error)

	// Validate validates Commitments against the corresponding Proofs. This should be possible without retrieving the
	// Blobs.
	//
	// Deprecated: This method is deprecated and will be removed in the future.
	Validate(ctx context.Context, ids []da.ID, proofs []da.Proof, namespace da.Namespace) ([]bool, error)
}

// API is a wrapper around Module for the RPC.
type API struct {
	Internal struct {
		MaxBlobSize       func(ctx context.Context) (uint64, error)                                            `perm:"read"`
		Get               func(ctx context.Context, ids []da.ID, ns da.Namespace) ([]da.Blob, error)           `perm:"read"`
		GetIDs            func(ctx context.Context, height uint64, ns da.Namespace) (*da.GetIDsResult, error)  `perm:"read"`
		GetProofs         func(ctx context.Context, ids []da.ID, ns da.Namespace) ([]da.Proof, error)          `perm:"read"`
		Commit            func(ctx context.Context, blobs []da.Blob, ns da.Namespace) ([]da.Commitment, error) `perm:"read"`
		Validate          func(context.Context, []da.ID, []da.Proof, da.Namespace) ([]bool, error)             `perm:"read"`
		Submit            func(context.Context, []da.Blob, float64, da.Namespace) ([]da.ID, error)             `perm:"write"`
		SubmitWithOptions func(context.Context, []da.Blob, float64, da.Namespace, []byte) ([]da.ID, error)     `perm:"write"`
	}
}

func (api *API) MaxBlobSize(ctx context.Context) (uint64, error) {
	return api.Internal.MaxBlobSize(ctx)
}

func (api *API) Get(ctx context.Context, ids []da.ID, ns da.Namespace) ([]da.Blob, error) {
	return api.Internal.Get(ctx, ids, ns)
}

func (api *API) GetIDs(ctx context.Context, height uint64, ns da.Namespace) (*da.GetIDsResult, error) {
	return api.Internal.GetIDs(ctx, height, ns)
}

func (api *API) GetProofs(ctx context.Context, ids []da.ID, ns da.Namespace) ([]da.Proof, error) {
	return api.Internal.GetProofs(ctx, ids, ns)
}

func (api *API) Commit(ctx context.Context, blobs []da.Blob, ns da.Namespace) ([]da.Commitment, error) {
	return api.Internal.Commit(ctx, blobs, ns)
}

func (api *API) Validate(ctx context.Context, ids []da.ID, proofs []da.Proof, ns da.Namespace) ([]bool, error) {
	return api.Internal.Validate(ctx, ids, proofs, ns)
}

func (api *API) Submit(ctx context.Context, blobs []da.Blob, gasPrice float64, ns da.Namespace) ([]da.ID, error) {
	return api.Internal.Submit(ctx, blobs, gasPrice, ns)
}

func (api *API) SubmitWithOptions(
	ctx context.Context,
	blobs []da.Blob,
	gasPrice float64,
	ns da.Namespace,
	options []byte,
) ([]da.ID, error) {
	return api.Internal.SubmitWithOptions(ctx, blobs, gasPrice, ns, options)
}
