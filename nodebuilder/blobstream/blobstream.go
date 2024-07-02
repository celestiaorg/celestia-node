package blobstream

import (
	"context"

	"github.com/celestiaorg/celestia-node/share"
)

var _ Module = (*API)(nil)

// Module defines the API related to interacting with the proofs
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// GetDataCommitment collects the data roots over a provided ordered range of blocks,
	// and then creates a new Merkle root of those data roots. The range is end exclusive.
	GetDataCommitment(ctx context.Context, start, end uint64) (*ResultDataCommitment, error)

	// GetDataRootInclusionProof creates an inclusion proof for the data root of block
	// height `height` in the set of blocks defined by `start` and `end`. The range
	// is end exclusive.
	GetDataRootInclusionProof(
		ctx context.Context,
		height int64,
		start, end uint64,
	) (*ResultDataRootInclusionProof, error)

	// ProveShares generates a share proof for a share range.
	ProveShares(ctx context.Context, height, start, end uint64) (*ResultShareProof, error)

	// ProveCommitment generates a commitment proof for a share commitment.
	ProveCommitment(
		ctx context.Context,
		height uint64,
		namespace share.Namespace,
		shareCommitment []byte,
	) (*ResultCommitmentProof, error)
}

// API is a wrapper around the Module for RPC.
type API struct {
	Internal struct {
		GetDataCommitment func(
			ctx context.Context,
			start, end uint64,
		) (*ResultDataCommitment, error) `perm:"read"`
		GetDataRootInclusionProof func(
			ctx context.Context,
			height int64,
			start, end uint64,
		) (*ResultDataRootInclusionProof, error) `perm:"read"`
		ProveShares func(
			ctx context.Context,
			height, start, end uint64,
		) (*ResultShareProof, error) `perm:"read"`
		ProveCommitment func(
			ctx context.Context,
			height uint64,
			namespace share.Namespace,
			shareCommitment []byte,
		) (*ResultCommitmentProof, error) `perm:"read"`
	}
}

func (api *API) GetDataCommitment(
	ctx context.Context,
	start, end uint64,
) (*ResultDataCommitment, error) {
	return api.Internal.GetDataCommitment(ctx, start, end)
}

func (api *API) GetDataRootInclusionProof(
	ctx context.Context,
	height int64,
	start, end uint64,
) (*ResultDataRootInclusionProof, error) {
	return api.Internal.GetDataRootInclusionProof(ctx, height, start, end)
}

func (api *API) ProveShares(
	ctx context.Context,
	height, start, end uint64,
) (*ResultShareProof, error) {
	return api.Internal.ProveShares(ctx, height, start, end)
}

func (api *API) ProveCommitment(
	ctx context.Context,
	height uint64,
	namespace share.Namespace,
	shareCommitment []byte,
) (*ResultCommitmentProof, error) {
	return api.Internal.ProveCommitment(ctx, height, namespace, shareCommitment)
}
