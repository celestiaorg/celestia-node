package share

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/types"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	headerServ "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var _ Module = (*API)(nil)

// RangeResult wraps the return value of the GetRange endpoint.
// It contains a set of shares along with their proof to
// the data root.
type RangeResult struct {
	// Shares the queried shares.
	Shares []libshare.Share `json:"shares"`
	// Proof the proof of Shares up to the data root.
	Proof *types.ShareProof `json:"proof"`
}

// Verify verifies if the shares and proof in the range
// are being committed to by the provided data root.
// Returns nil if the proof is valid and a sensible error otherwise.
func (r RangeResult) Verify(dataRoot []byte) error {
	if len(dataRoot) == 0 {
		return errors.New("root must be non-empty")
	}

	for index, data := range r.Shares {
		if !bytes.Equal(data.ToBytes(), r.Proof.Data[index]) {
			return fmt.Errorf("mismatching share %d between the range result and the proof", index)
		}
	}
	return r.Proof.Validate(dataRoot)
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
	SharesAvailable(ctx context.Context, height uint64) error
	// GetShare gets a Share by coordinates in EDS.
	GetShare(ctx context.Context, height uint64, row, col int) (libshare.Share, error)
	// GetEDS gets the full EDS identified by the given extended header.
	GetEDS(ctx context.Context, height uint64) (*rsmt2d.ExtendedDataSquare, error)
	// GetNamespaceData gets all shares from an EDS within the given namespace.
	// Shares are returned in a row-by-row order if the namespace spans multiple rows.
	GetNamespaceData(
		ctx context.Context, height uint64, namespace libshare.Namespace,
	) (shwap.NamespaceData, error)
	// GetRange gets a list of shares and their corresponding proof.
	GetRange(ctx context.Context, height uint64, start, end int) (*RangeResult, error)
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
		GetEDS func(
			ctx context.Context,
			height uint64,
		) (*rsmt2d.ExtendedDataSquare, error) `perm:"read"`
		GetNamespaceData func(
			ctx context.Context,
			height uint64,
			namespace libshare.Namespace,
		) (shwap.NamespaceData, error) `perm:"read"`
		GetRange func(
			ctx context.Context,
			height uint64,
			start, end int,
		) (*RangeResult, error) `perm:"read"`
	}
}

func (api *API) SharesAvailable(ctx context.Context, height uint64) error {
	return api.Internal.SharesAvailable(ctx, height)
}

func (api *API) GetShare(ctx context.Context, height uint64, row, col int) (libshare.Share, error) {
	return api.Internal.GetShare(ctx, height, row, col)
}

func (api *API) GetEDS(ctx context.Context, height uint64) (*rsmt2d.ExtendedDataSquare, error) {
	return api.Internal.GetEDS(ctx, height)
}

func (api *API) GetRange(ctx context.Context, height uint64, start, end int) (*RangeResult, error) {
	return api.Internal.GetRange(ctx, height, start, end)
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
	return m.getter.GetShare(ctx, header, row, col)
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

func (m module) GetRange(ctx context.Context, height uint64, start, end int) (*RangeResult, error) {
	extendedDataSquare, err := m.GetEDS(ctx, height)
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
	return &RangeResult{
		Shares: shares,
		Proof:  proof,
	}, nil
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
