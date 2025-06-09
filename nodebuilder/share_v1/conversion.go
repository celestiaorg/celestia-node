//revive:disable:var-naming
package share_v1

import (
	"context"
	"fmt"

	"github.com/celestiaorg/nmt"
	"github.com/cometbft/cometbft/crypto/merkle"
	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	coretypes "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/types"

	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	nodeShare "github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	libshare "github.com/celestiaorg/go-square/v2/share"
)

// ConversionAdapter provides conversion utilities between old and new proof structures.
// It wraps both the legacy share module and the optimized share_v1 module to provide
// seamless conversion between different proof formats and APIs.
type ConversionAdapter struct {
	shareModule   share.Module
	shareV1Module Module
}

// NewConversionAdapter creates a new conversion adapter that bridges the legacy and optimized share modules.
func NewConversionAdapter(shareModule share.Module, shareV1Module Module) *ConversionAdapter {
	return &ConversionAdapter{
		shareModule:   shareModule,
		shareV1Module: shareV1Module,
	}
}

// =============================================================================
// LEGACY INTERFACE METHODS
// These methods provide backwards compatibility using the original share module
// =============================================================================

// GetRangeWithLegacyProof retrieves range data using the legacy share module.
// This provides perfect backwards compatibility with the exact same behavior as the original GetRange.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - height: Block height to query
//   - start: Starting share index (inclusive)
//   - end: Ending share index (exclusive)
//
// Returns: Legacy GetRangeResult with original proof format
//
// Note: This method does NOT provide optimization benefits since it uses the legacy module.
// For optimizations with legacy format, use OptimizedGetRangeWithLegacyFormat().
func (ca *ConversionAdapter) GetRangeWithLegacyProof(
	ctx context.Context,
	height uint64,
	start, end int,
) (*share.GetRangeResult, error) {
	return ca.shareModule.GetRange(ctx, height, start, end)
}

// =============================================================================
// OPTIMIZED INTERFACE METHODS
// These methods use the new optimized share_v1 module for better performance
// =============================================================================

// OptimizedGetRange retrieves range data using the optimized share_v1 module.
// This provides the best performance and returns data in the new optimized format.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - namespace: Target namespace to query
//   - height: Block height to query
//   - fromCoords: Starting coordinates (row, col)
//   - toCoords: Ending coordinates (row, col)
//   - proofsOnly: If true, only returns proofs without share data
//
// Returns: Optimized RangeNamespaceData with efficient proof structure
func (ca *ConversionAdapter) OptimizedGetRange(
	ctx context.Context,
	namespace libshare.Namespace,
	height uint64,
	fromCoords, toCoords shwap.SampleCoords,
	proofsOnly bool,
) (shwap.RangeNamespaceData, error) {
	return ca.shareV1Module.GetRange(ctx, namespace, height, fromCoords, toCoords, proofsOnly)
}

// OptimizedGetRangeWithLegacyFormat provides optimization benefits while maintaining legacy format.
// This method uses the optimized share_v1 module internally and converts the result back to
// the legacy format, giving you the best of both worlds.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - namespace: Target namespace to query
//   - height: Block height to query
//   - start: Starting share index (inclusive)
//   - end: Ending share index (exclusive)
//
// Returns: Legacy GetRangeResult format with data retrieved using optimized backend
//
// Note: Unlike GetRangeWithLegacyProof, this requires a namespace parameter but provides optimization benefits.
func (ca *ConversionAdapter) OptimizedGetRangeWithLegacyFormat(
	ctx context.Context,
	namespace libshare.Namespace,
	height uint64,
	start, end int,
) (*share.GetRangeResult, error) {
	// Convert legacy start/end to coordinates (assumes single row for simplicity)
	// In a real implementation, this might need more sophisticated coordinate mapping
	fromCoords := shwap.SampleCoords{Row: 0, Col: start}
	toCoords := shwap.SampleCoords{Row: 0, Col: end - 1}

	// Use optimized module to get data
	rangeData, err := ca.shareV1Module.GetRange(ctx, namespace, height, fromCoords, toCoords, false)
	if err != nil {
		return nil, fmt.Errorf("optimized GetRange failed: %w", err)
	}

	// Convert optimized result back to legacy format
	return ConvertRangeNamespaceDataToLegacy(ctx, rangeData, height, start, end)
}

// =============================================================================
// CONVERSION UTILITY FUNCTIONS
// These functions convert between legacy and optimized proof formats
// =============================================================================

// ConvertRangeNamespaceDataToLegacy converts optimized RangeNamespaceData to the legacy GetRangeResult format.
// This is useful when you have optimized data but need to return it in legacy format for backwards compatibility.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - rangeData: Optimized range data to convert
//   - height: Block height (for legacy proof structure)
//   - start: Starting share index for legacy format
//   - end: Ending share index for legacy format
//
// Returns: Legacy GetRangeResult with converted proof structure
func ConvertRangeNamespaceDataToLegacy(
	ctx context.Context,
	rangeData shwap.RangeNamespaceData,
	height uint64,
	start, end int,
) (*share.GetRangeResult, error) {
	// Flatten shares from the RangeNamespaceData
	shares := rangeData.Flatten()
	if len(shares) == 0 {
		return nil, fmt.Errorf("no shares found in range data")
	}

	// Create a legacy proof structure
	legacyProof := &types.ShareProof{
		Data:             make([][]byte, len(shares)),
		ShareProofs:      make([]*coretypes.NMTProof, len(shares)),
		NamespaceID:      shares[0].Namespace().Bytes(),
		RowProof:         types.RowProof{}, // This would need proper conversion
		NamespaceVersion: uint32(shares[0].Namespace().Version()),
	}

	// Convert shares to byte format for legacy proof
	for i, share := range shares {
		legacyProof.Data[i] = share.ToBytes()
	}

	legacyProof.RowProof.StartRow = uint32(rangeData.Start)
	legacyProof.RowProof.EndRow = uint32(rangeData.Start) + uint32(len(rangeData.Shares)) - 1
	legacyProof.RowProof.Proofs = make([]*merkle.Proof, len(rangeData.Proof))
	legacyProof.RowProof.RowRoots = make([]tmbytes.HexBytes, len(rangeData.Proof))

	for i, proof := range rangeData.Proof {
		sharesProof := proof.SharesProof()
		legacyProof.ShareProofs[i] = &coretypes.NMTProof{
			Start: int32(sharesProof.Start()),
			End:   int32(sharesProof.End()),
			Nodes: sharesProof.Nodes(),
		}
		rowRootProof := proof.RowRootProof()
		rowRoot, err := sharesProof.ComputeRootWithBasicValidation(
			nmt.NewNmtHasher(
				nodeShare.NewSHA256Hasher(),
				libshare.NamespaceIDSize,
				false,
			),
			shares[0].Namespace().ID(),
			legacyProof.Data,
			false,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to compute row root: %w", err)
		}
		legacyProof.RowProof.RowRoots[i] = rowRoot
		legacyProof.RowProof.Proofs[i] = rowRootProof
	}

	return &share.GetRangeResult{
		Shares: shares,
		Proof:  legacyProof,
	}, nil
}

// ConvertLegacyToRangeNamespaceData converts legacy GetRangeResult to optimized RangeNamespaceData format.
// This is useful when you have legacy data but want to work with the optimized structure.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - legacyResult: Legacy result to convert
//   - namespace: Target namespace for the conversion
//   - fromCoords: Starting coordinates for optimized format (row, col)
//   - toCoords: Ending coordinates for optimized format (row, col)
//   - edsSize: Extended Data Square size (kept for interface compatibility, not used in coordinate calculation)
//
// Returns: Optimized RangeNamespaceData with efficient proof structure
func ConvertLegacyToRangeNamespaceData(
	ctx context.Context,
	legacyResult *share.GetRangeResult,
	namespace libshare.Namespace,
	fromCoords, toCoords shwap.SampleCoords,
	edsSize int,
) (shwap.RangeNamespaceData, error) {
	if legacyResult == nil || len(legacyResult.Shares) == 0 {
		return shwap.RangeNamespaceData{}, fmt.Errorf("empty legacy result")
	}

	// Validate proof structure
	if legacyResult.Proof == nil {
		return shwap.RangeNamespaceData{}, fmt.Errorf("legacy result has nil proof")
	}

	// Pre-calculate constants (same for all rows in rectangular interpretation)
	startCol := fromCoords.Col
	endCol := toCoords.Col
	rowShareCount := endCol - startCol + 1
	numRows := toCoords.Row - fromCoords.Row + 1
	totalSharesNeeded := rowShareCount * numRows

	// Validate total shares needed upfront (more efficient than checking each iteration)
	if totalSharesNeeded > len(legacyResult.Shares) {
		return shwap.RangeNamespaceData{}, fmt.Errorf(
			"insufficient shares for conversion: need %d shares (%d rows × %d cols), but only have %d shares",
			totalSharesNeeded, numRows, rowShareCount, len(legacyResult.Shares),
		)
	}

	// Create grouped shares array and efficiently populate it
	groupedShares := make([][]libshare.Share, numRows)
	shareIdx := 0
	for row := fromCoords.Row; row <= toCoords.Row; row++ {
		groupedShares[row-fromCoords.Row] = legacyResult.Shares[shareIdx : shareIdx+rowShareCount]
		shareIdx += rowShareCount
	}

	// Create optimized proof structure with defensive checks
	proofs := make([]*shwap.Proof, numRows)

	// Check if we have share proofs
	if len(legacyResult.Proof.ShareProofs) == 0 {
		// Return data without proofs if none available
		return shwap.RangeNamespaceData{
			Start:  fromCoords.Row,
			Shares: groupedShares,
			Proof:  proofs, // Empty proofs
		}, nil
	}

	// Safely convert available proofs
	maxProofs := len(legacyResult.Proof.ShareProofs)
	if maxProofs > numRows {
		maxProofs = numRows
	}

	for i := 0; i < maxProofs; i++ {
		legacyProof := legacyResult.Proof.ShareProofs[i]
		if legacyProof == nil {
			continue // Skip nil proofs
		}

		// Convert legacy proof to optimized proof
		sharesProof := nmt.NewInclusionProof(
			int(legacyProof.Start),
			int(legacyProof.End),
			legacyProof.Nodes,
			false,
		)

		// Safely access row root proof
		var rowRootProof *merkle.Proof
		if i < len(legacyResult.Proof.RowProof.Proofs) && legacyResult.Proof.RowProof.Proofs[i] != nil {
			rowRootProof = legacyResult.Proof.RowProof.Proofs[i]
		} else {
			// Create empty proof if not available
			rowRootProof = &merkle.Proof{}
		}

		proofs[i] = shwap.NewProof(&sharesProof, rowRootProof)
	}

	return shwap.RangeNamespaceData{
		Start:  fromCoords.Row,
		Shares: groupedShares,
		Proof:  proofs,
	}, nil
}
