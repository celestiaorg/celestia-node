package blobstream

import (
	"context"
)

var _ Module = (*API)(nil)

// Module defines the API related to interacting with the data root tuples proofs
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// GetDataRootTupleRoot collects the data roots over a provided ordered range of blocks,
	// and then creates a new Merkle root of those data roots. The range is end exclusive.
	// It's in the header module because it only needs access to the headers to generate the proof.
	GetDataRootTupleRoot(ctx context.Context, start, end uint64) (*DataRootTupleRoot, error)

	// GetDataRootTupleInclusionProof creates an inclusion proof, for the data root tuple of block
	// height `height`, in the set of blocks defined by `start` and `end`. The range
	// is end exclusive.
	// It's in the header module because it only needs access to the headers to generate the proof.
	GetDataRootTupleInclusionProof(
		ctx context.Context,
		height, start, end uint64,
	) (*DataRootTupleInclusionProof, error)
}

// API is a wrapper around the Module for RPC.
type API struct {
	Internal struct {
		GetDataRootTupleRoot           func(ctx context.Context, start, end uint64) (*DataRootTupleRoot, error) `perm:"read"`
		GetDataRootTupleInclusionProof func(
			ctx context.Context,
			height, start, end uint64,
		) (*DataRootTupleInclusionProof, error) `perm:"read"`
	}
}

func (api *API) GetDataRootTupleRoot(ctx context.Context, start, end uint64) (*DataRootTupleRoot, error) {
	return api.Internal.GetDataRootTupleRoot(ctx, start, end)
}

func (api *API) GetDataRootTupleInclusionProof(
	ctx context.Context,
	height, start, end uint64,
) (*DataRootTupleInclusionProof, error) {
	return api.Internal.GetDataRootTupleInclusionProof(ctx, height, start, end)
}
