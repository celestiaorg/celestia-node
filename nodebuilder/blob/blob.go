package blob

import (
	"context"

	"github.com/celestiaorg/nmt/namespace"

	"github.com/celestiaorg/celestia-node/blob"
)

var _ Module = (*API)(nil)

// Module defines the API related to interacting with the blobs
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// Submit sends PFB transaction and reports the height it landed on.
	// Allows sending multiple Blobs atomically synchronously.
	// Uses default wallet registered on the Node.
	Submit(_ context.Context, fee int64, gasLimit uint64, _ ...*blob.Blob) (height uint64, _ error)
	// Get retrieves the blob by commitment.
	Get(_ context.Context, height uint64, _ namespace.ID, _ blob.Commitment) (*blob.Blob, error)
	// GetAll retrieves all the blobs for given namespaces at the given height.
	GetAll(_ context.Context, height uint64, _ ...namespace.ID) ([]*blob.Blob, error)
	// GetProof gets the Proof for a blob if it exists at the given height under the namespace.
	GetProof(_ context.Context, height uint64, _ namespace.ID, _ blob.Commitment) (*blob.Proof, error)
	// Included checks whether a blob's given commitment(Merkle subtree root) is included at the
	// given height under a particular namespace(Useful for light clients wishing to check the
	// inclusion of the data).
	Included(_ context.Context, height uint64, _ namespace.ID, _ *blob.Proof, _ blob.Commitment) (bool, error)
}

type API struct {
	Internal struct {
		Submit   func(context.Context, int64, uint64, ...*blob.Blob) (uint64, error)                     `perm:"write"`
		Get      func(context.Context, uint64, namespace.ID, blob.Commitment) (*blob.Blob, error)        `perm:"read"`
		GetAll   func(context.Context, uint64, ...namespace.ID) ([]*blob.Blob, error)                    `perm:"read"`
		GetProof func(context.Context, uint64, namespace.ID, blob.Commitment) (*blob.Proof, error)       `perm:"read"`
		Included func(context.Context, uint64, namespace.ID, *blob.Proof, blob.Commitment) (bool, error) `perm:"read"`
	}
}

func (api *API) Submit(ctx context.Context, fee int64, gasLimit uint64, blobs ...*blob.Blob) (uint64, error) {
	return api.Internal.Submit(ctx, fee, gasLimit, blobs...)
}

func (api *API) Get(
	ctx context.Context,
	height uint64,
	nID namespace.ID,
	commitment blob.Commitment,
) (*blob.Blob, error) {
	return api.Internal.Get(ctx, height, nID, commitment)
}

func (api *API) GetAll(ctx context.Context, height uint64, nIDs ...namespace.ID) ([]*blob.Blob, error) {
	return api.Internal.GetAll(ctx, height, nIDs...)
}

func (api *API) GetProof(
	ctx context.Context,
	height uint64,
	nID namespace.ID,
	commitment blob.Commitment,
) (*blob.Proof, error) {
	return api.Internal.GetProof(ctx, height, nID, commitment)
}

func (api *API) Included(
	ctx context.Context,
	height uint64,
	nID namespace.ID,
	proof *blob.Proof,
	commitment blob.Commitment,
) (bool, error) {
	return api.Internal.Included(ctx, height, nID, proof, commitment)
}
