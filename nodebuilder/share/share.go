package share

import (
	"context"
	"encoding/json"

	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/types"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

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

// MarshalJSON marshals an GetRangeResult to JSON. Uses tendermint encoder for proof for compatibility.
func (getRangeResult *GetRangeResult) MarshalJSON() ([]byte, error) {
	type Alias GetRangeResult
	proof, err := tmjson.Marshal(getRangeResult.Proof)
	if err != nil {
		return nil, err
	}
	return json.Marshal(&struct {
		Proof json.RawMessage `json:"Proof"`
		*Alias
	}{
		Proof: proof,
		Alias: (*Alias)(getRangeResult),
	})
}

// UnmarshalJSON unmarshals an GetRangeResult from JSON. Uses tendermint decoder for proof for compatibility.
func (getRangeResult *GetRangeResult) UnmarshalJSON(data []byte) error {
	type Alias GetRangeResult
	aux := &struct {
		Proof json.RawMessage `json:"Proof"`
		*Alias
	}{
		Alias: (*Alias)(getRangeResult),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	proof := new(types.ShareProof)
	if err := tmjson.Unmarshal(aux.Proof, proof); err != nil {
		return err
	}

	getRangeResult.Proof = proof

	return nil
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
	GetRange(ctx context.Context, height uint64, start, end int) (*GetRangeResult, error)
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
		) (*GetRangeResult, error) `perm:"read"`
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

func (api *API) GetRange(ctx context.Context, height uint64, start, end int) (*GetRangeResult, error) {
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

func (m module) GetRange(ctx context.Context, height uint64, start, end int) (*GetRangeResult, error) {
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
	return &GetRangeResult{
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
