package blob

import (
	"context"

	libshare "github.com/celestiaorg/go-square/v2/share"

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
	Submit(_ context.Context, _ []*blob.Blob, _ *blob.SubmitOptions) (height uint64, _ error)
	// Get retrieves the blob by commitment under the given namespace and height.
	Get(_ context.Context, height uint64, _ libshare.Namespace, _ blob.Commitment) (*blob.Blob, error)
	// GetAll returns all blobs under the given namespaces at the given height.
	// If all blobs were found without any errors, the user will receive a list of blobs.
	// If the BlobService couldn't find any blobs under the requested namespaces,
	// the user will receive an empty list of blobs along with an empty error.
	// If some of the requested namespaces were not found, the user will receive all the found blobs
	// and an empty error. If there were internal errors during some of the requests,
	// the user will receive all found blobs along with a combined error message.
	//
	// All blobs will preserve the order of the namespaces that were requested.
	GetAll(_ context.Context, height uint64, _ []libshare.Namespace) ([]*blob.Blob, error)
	// GetProof retrieves proofs in the given namespaces at the given height by commitment.
	GetProof(_ context.Context, height uint64, _ libshare.Namespace, _ blob.Commitment) (*blob.Proof, error)
	// Included checks whether a blob's given commitment(Merkle subtree root) is included at
	// given height and under the namespace.
	Included(_ context.Context, height uint64, _ libshare.Namespace, _ *blob.Proof, _ blob.Commitment) (bool, error)
	// Subscribe to published blobs from the given namespace as they are included.
	Subscribe(_ context.Context, _ libshare.Namespace) (<-chan *blob.SubscriptionResponse, error)
}

type API struct {
	Internal struct {
		Submit func(
			context.Context,
			[]*blob.Blob,
			*blob.SubmitOptions,
		) (uint64, error) `perm:"write"`
		Get func(
			context.Context,
			uint64,
			libshare.Namespace,
			blob.Commitment,
		) (*blob.Blob, error) `perm:"read"`
		GetAll func(
			context.Context,
			uint64,
			[]libshare.Namespace,
		) ([]*blob.Blob, error) `perm:"read"`
		GetProof func(
			context.Context,
			uint64,
			libshare.Namespace,
			blob.Commitment,
		) (*blob.Proof, error) `perm:"read"`
		Included func(
			context.Context,
			uint64,
			libshare.Namespace,
			*blob.Proof,
			blob.Commitment,
		) (bool, error) `perm:"read"`
		Subscribe func(
			context.Context,
			libshare.Namespace,
		) (<-chan *blob.SubscriptionResponse, error) `perm:"read"`
	}
}

func (api *API) Submit(ctx context.Context, blobs []*blob.Blob, options *blob.SubmitOptions) (uint64, error) {
	return api.Internal.Submit(ctx, blobs, options)
}

func (api *API) Get(
	ctx context.Context,
	height uint64,
	namespace libshare.Namespace,
	commitment blob.Commitment,
) (*blob.Blob, error) {
	return api.Internal.Get(ctx, height, namespace, commitment)
}

func (api *API) GetAll(ctx context.Context, height uint64, namespaces []libshare.Namespace) ([]*blob.Blob, error) {
	return api.Internal.GetAll(ctx, height, namespaces)
}

func (api *API) GetProof(
	ctx context.Context,
	height uint64,
	namespace libshare.Namespace,
	commitment blob.Commitment,
) (*blob.Proof, error) {
	return api.Internal.GetProof(ctx, height, namespace, commitment)
}

func (api *API) Included(
	ctx context.Context,
	height uint64,
	namespace libshare.Namespace,
	proof *blob.Proof,
	commitment blob.Commitment,
) (bool, error) {
	return api.Internal.Included(ctx, height, namespace, proof, commitment)
}

func (api *API) Subscribe(
	ctx context.Context,
	namespace libshare.Namespace,
) (<-chan *blob.SubscriptionResponse, error) {
	return api.Internal.Subscribe(ctx, namespace)
}
