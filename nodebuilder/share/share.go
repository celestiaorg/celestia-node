package share

import (
	"context"

	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	headerServ "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ Module = (*API)(nil)

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
	// SharesAvailable performs a subjective validation to check if the shares committed to
	// the ExtendedHeader at the specified height are available and retrievable from the network.
	// Returns an error if the shares are not available or if validation fails.
	SharesAvailable(ctx context.Context, height uint64) error

	// GetShare retrieves a specific share from the Extended Data Square (EDS) at the given height
	// using its row and column coordinates. Returns the share data or an error if retrieval fails.
	GetShare(ctx context.Context, height uint64, rowIdx, colIdx int) (libshare.Share, error)

	// GetSamples retrieves multiple shares from the Extended Data Square (EDS) specified by the header
	// at the given sample coordinates. Returns an array of samples containing the requested shares
	// or an error if retrieval fails.
	GetSamples(ctx context.Context, height uint64, indices []shwap.SampleCoords) ([]shwap.Sample, error)

	// GetEDS retrieves the complete Extended Data Square (EDS) for the specified height.
	// The EDS contains all shares organized in a 2D matrix format with erasure coding.
	// Returns the full EDS or an error if retrieval fails.
	GetEDS(ctx context.Context, height uint64) (*rsmt2d.ExtendedDataSquare, error)

	// GetRow retrieves all shares from a specific row in the Extended Data Square (EDS)
	// at the given height. Returns the complete row of shares or an error if retrieval fails.
	GetRow(ctx context.Context, height uint64, rowIdx int) (shwap.Row, error)

	// GetNamespaceData retrieves all shares that belong to the specified namespace within
	// the Extended Data Square (EDS) at the given height. The shares are returned in a
	// row-by-row order, maintaining the original layout if the namespace spans multiple rows.
	// Returns the namespace data or an error if retrieval fails.
	GetNamespaceData(
		ctx context.Context,
		height uint64,
		namespace libshare.Namespace,
	) (shwap.NamespaceData, error)

	// GetRange retrieves a range of the *original* shares and their corresponding proofs within a specific
	// namespace in the Extended Data Square (EDS) at the given height. The range is defined
	// by start and end indexes. Returns the range data with proof to the data root or an error if retrieval fails.
	GetRange(
		ctx context.Context,
		height uint64,
		start, end int,
	) (*GetRangeResult, error)
}

// API is a wrapper around Module for the RPC.
type API struct {
	Internal struct {
		SharesAvailable func(ctx context.Context, height uint64) error `perm:"read"`
		GetShare        func(
			ctx context.Context,
			height uint64,
			row, col int,
		) (libshare.Share, error) `perm:"read"`
		GetSamples func(
			ctx context.Context,
			height uint64,
			indices []shwap.SampleCoords,
		) ([]shwap.Sample, error) `perm:"read"`
		GetEDS func(
			ctx context.Context,
			height uint64,
		) (*rsmt2d.ExtendedDataSquare, error) `perm:"read"`
		GetRow func(
			context.Context,
			uint64,
			int,
		) (shwap.Row, error) `perm:"read"`
		GetNamespaceData func(
			ctx context.Context,
			height uint64,
			namespace libshare.Namespace,
		) (shwap.NamespaceData, error) `perm:"read"`
		GetRange func(
			ctx context.Context,
			height uint64,
			start, end int,
		) (*GetRangeResult, error) `perm:"read"`
	}
}

func (api *API) SharesAvailable(ctx context.Context, height uint64) error {
	return api.Internal.SharesAvailable(ctx, height)
}

func (api *API) GetShare(ctx context.Context, height uint64, row, col int) (libshare.Share, error) {
	return api.Internal.GetShare(ctx, height, row, col)
}

func (api *API) GetSamples(ctx context.Context, height uint64,
	indices []shwap.SampleCoords,
) ([]shwap.Sample, error) {
	return api.Internal.GetSamples(ctx, height, indices)
}

func (api *API) GetEDS(ctx context.Context, height uint64) (*rsmt2d.ExtendedDataSquare, error) {
	return api.Internal.GetEDS(ctx, height)
}

func (api *API) GetRow(ctx context.Context, height uint64, rowIdx int) (shwap.Row, error) {
	return api.Internal.GetRow(ctx, height, rowIdx)
}

func (api *API) GetRange(ctx context.Context, height uint64, from, to int,
) (*GetRangeResult, error) {
	return api.Internal.GetRange(ctx, height, from, to)
}

func (api *API) GetNamespaceData(
	ctx context.Context,
	height uint64,
	namespace libshare.Namespace,
) (shwap.NamespaceData, error) {
	return api.Internal.GetNamespaceData(ctx, height, namespace)
}

type module struct {
	getter shwap.Getter
	avail  share.Availability
	hs     headerServ.Module
}

func (m module) GetShare(ctx context.Context, height uint64, row, col int) (libshare.Share, error) {
	header, err := m.hs.GetByHeight(ctx, height)
	if err != nil {
		return libshare.Share{}, err
	}

	idx := shwap.SampleCoords{Row: row, Col: col}

	smpls, err := m.getter.GetSamples(ctx, header, []shwap.SampleCoords{idx})
	if err != nil {
		return libshare.Share{}, err
	}

	return smpls[0].Share, nil
}

func (m module) GetSamples(ctx context.Context, height uint64,
	indices []shwap.SampleCoords,
) ([]shwap.Sample, error) {
	header, err := m.hs.GetByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	return m.getter.GetSamples(ctx, header, indices)
}

func (m module) GetEDS(ctx context.Context, height uint64) (*rsmt2d.ExtendedDataSquare, error) {
	header, err := m.hs.GetByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	return m.getter.GetEDS(ctx, header)
}

func (m module) SharesAvailable(ctx context.Context, height uint64) error {
	header, err := m.hs.GetByHeight(ctx, height)
	if err != nil {
		return err
	}
	return m.avail.SharesAvailable(ctx, header)
}

func (m module) GetRange(
	ctx context.Context,
	height uint64,
	start, end int,
) (*GetRangeResult, error) {
	hdr, err := m.hs.GetByHeight(ctx, height)
	if err != nil {
		return nil, err
	}

	rngData, err := m.getter.GetRangeNamespaceData(ctx, hdr, start, end)
	if err != nil {
		return nil, err
	}
	return newGetRangeResult(start, end, &rngData, hdr.DAH)
}

func (m module) GetNamespaceData(
	ctx context.Context,
	height uint64,
	namespace libshare.Namespace,
) (shwap.NamespaceData, error) {
	header, err := m.hs.GetByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	return m.getter.GetNamespaceData(ctx, header, namespace)
}

func (m module) GetRow(ctx context.Context, height uint64, rowIdx int) (shwap.Row, error) {
	header, err := m.hs.GetByHeight(ctx, height)
	if err != nil {
		return shwap.Row{}, err
	}
	return m.getter.GetRow(ctx, header, rowIdx)
}
