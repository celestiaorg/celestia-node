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
	// Submit sends Blobs and reports the height in which they were included.
	// Allows sending multiple Blobs atomically synchronously.
	// Uses default wallet registered on the Node.
	Submit(_ context.Context, _ ...*blob.Blob) (height uint64, _ error)
	// Get retrieves the blob by commitment under the given namespace and height.
	Get(_ context.Context, height uint64, _ namespace.ID, _ blob.Commitment) (*blob.Blob, error)
	// GetAll returns all blobs under the given namespaces and height.
	GetAll(_ context.Context, height uint64, _ ...namespace.ID) ([]*blob.Blob, error)
	// GetProof retrieves proofs in the given namespaces at the given height by commitment.
	GetProof(_ context.Context, height uint64, _ namespace.ID, _ blob.Commitment) (*blob.Proof, error)
	// Included checks whether a blob's given commitment(Merkle subtree root) is included at
	// given height and under the namespace.
	Included(_ context.Context, height uint64, _ namespace.ID, _ *blob.Proof, _ blob.Commitment) (bool, error)
}

type API struct {
	Internal struct {
		Submit   func(context.Context, ...*blob.Blob) (uint64, error)                                    `perm:"write"`
		Get      func(context.Context, uint64, namespace.ID, blob.Commitment) (*blob.Blob, error)        `perm:"read"`
		GetAll   func(context.Context, uint64, ...namespace.ID) ([]*blob.Blob, error)                    `perm:"read"`
		GetProof func(context.Context, uint64, namespace.ID, blob.Commitment) (*blob.Proof, error)       `perm:"read"`
		Included func(context.Context, uint64, namespace.ID, *blob.Proof, blob.Commitment) (bool, error) `perm:"read"`
	}
}

func (api *API) Submit(ctx context.Context, blobs ...*blob.Blob) (uint64, error) {
	return api.Internal.Submit(ctx, blobs...)
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
