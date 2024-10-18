package share

import (
	"context"

	"github.com/tendermint/tendermint/types"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	headerServ "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ Module = (*API)(nil)

// GetRangeResult wraps the return value of the GetRange endpoint
// because Json-RPC doesn't support more than two return values.
type GetRangeResult struct {
	Shares []libshare.Share
	Proof  *types.ShareProof
}

// Module provides access to any data square or block share on the network.
//
// All Get methods provided on Module follow the following flow:
//  1. Check local storage for the requested share.
//  2. If exists
//     * Load from disk
//     * Return
//  3. If not
//     * Find provider on the network
//     * Fetch the Share from the provider
//     * Store the Share
//     * Return
//
// Any method signature changed here needs to also be changed in the API struct.
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// SharesAvailable subjectively validates if Shares committed to the given
	// ExtendedHeader are available on the Network.
	SharesAvailable(context.Context, *header.ExtendedHeader) error
	// GetShare gets a Share by coordinates in EDS.
	GetShare(ctx context.Context, header *header.ExtendedHeader, row, col int) (libshare.Share, error)
	// GetEDS gets the full EDS identified by the given extended header.
	GetEDS(ctx context.Context, header *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error)
	// GetSharesByNamespace gets all shares from an EDS within the given namespace.
	// Shares are returned in a row-by-row order if the namespace spans multiple rows.
	GetSharesByNamespace(
		ctx context.Context, header *header.ExtendedHeader, namespace libshare.Namespace,
	) (shwap.NamespaceData, error)
	// GetRange gets a list of shares and their corresponding proof.
	GetRange(ctx context.Context, height uint64, start, end int) (*GetRangeResult, error)
}

// API is a wrapper around Module for the RPC.
type API struct {
	Internal struct {
		SharesAvailable func(context.Context, *header.ExtendedHeader) error `perm:"read"`
		GetShare        func(
			ctx context.Context,
			header *header.ExtendedHeader,
			row, col int,
		) (libshare.Share, error) `perm:"read"`
		GetEDS func(
			ctx context.Context,
			header *header.ExtendedHeader,
		) (*rsmt2d.ExtendedDataSquare, error) `perm:"read"`
		GetSharesByNamespace func(
			ctx context.Context,
			header *header.ExtendedHeader,
			namespace libshare.Namespace,
		) (shwap.NamespaceData, error) `perm:"read"`
		GetRange func(
			ctx context.Context,
			height uint64,
			start, end int,
		) (*GetRangeResult, error) `perm:"read"`
	}
}

func (api *API) SharesAvailable(ctx context.Context, header *header.ExtendedHeader) error {
	return api.Internal.SharesAvailable(ctx, header)
}

func (api *API) GetShare(ctx context.Context, header *header.ExtendedHeader, row, col int) (libshare.Share, error) {
	return api.Internal.GetShare(ctx, header, row, col)
}

func (api *API) GetEDS(ctx context.Context, header *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
	return api.Internal.GetEDS(ctx, header)
}

func (api *API) GetRange(ctx context.Context, height uint64, start, end int) (*GetRangeResult, error) {
	return api.Internal.GetRange(ctx, height, start, end)
}

func (api *API) GetSharesByNamespace(
	ctx context.Context,
	header *header.ExtendedHeader,
	namespace libshare.Namespace,
) (shwap.NamespaceData, error) {
	return api.Internal.GetSharesByNamespace(ctx, header, namespace)
}

type module struct {
	shwap.Getter
	share.Availability
	hs headerServ.Module
}

func (m module) SharesAvailable(ctx context.Context, header *header.ExtendedHeader) error {
	return m.Availability.SharesAvailable(ctx, header)
}

func (m module) GetRange(ctx context.Context, height uint64, start, end int) (*GetRangeResult, error) {
	extendedHeader, err := m.hs.GetByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	extendedDataSquare, err := m.GetEDS(ctx, extendedHeader)
	if err != nil {
		return nil, err
	}

	proof, err := eds.ProveShares(extendedDataSquare, start, end)
	if err != nil {
		return nil, err
	}

	shares, err := libshare.FromBytes(extendedDataSquare.FlattenedODS()[start:end])
	if err != nil {
		return nil, err
	}
	return &GetRangeResult{
		Shares: shares,
		Proof:  proof,
	}, nil
}

func (m module) GetSharesByNamespace(
	ctx context.Context,
	header *header.ExtendedHeader,
	namespace libshare.Namespace,
) (shwap.NamespaceData, error) {
	return m.Getter.GetSharesByNamespace(ctx, header, namespace)
}
