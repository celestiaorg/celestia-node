package cda

import (
	"context"
	"errors"
	"fmt"

	blsfr "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// Availability implements share.Availability using CDA fragments + header commitments.
//
// This is an incremental bridge: it reconstructs per-sample coordinate from k fragments stored
// by the local CDA node and treats successful reconstruction as "available".
// Merkle proofs and fraud proofs are intentionally bypassed in the CDA path.
type Availability struct {
	node        *share.CDANode
	sampleCount int
	k           int
}

type Config struct {
	// SampleAmount is the number of random coordinates to check per header.
	SampleAmount int
	// K is the number of fragments required for reconstruction.
	K int
}

func New(node *share.CDANode, cfg Config) *Availability {
	if cfg.SampleAmount <= 0 {
		cfg.SampleAmount = 16
	}
	if cfg.K <= 0 {
		cfg.K = 16
	}
	return &Availability{
		node:        node,
		sampleCount: cfg.SampleAmount,
		k:           cfg.K,
	}
}

func (a *Availability) SharesAvailable(ctx context.Context, hdr *header.ExtendedHeader) error {
	if hdr == nil {
		return share.ErrNotAvailable
	}
	if a == nil || a.node == nil || a.node.Service() == nil {
		return errors.New("cda: missing CDA node/service")
	}
	if a.node.Verifier() == nil || a.node.CommitmentSource() == nil {
		return errors.New("cda: missing verifier or commitment source")
	}

	// short-circuit if outside sampling window (same policy as light availability)
	if !availability.IsWithinWindow(hdr.Time(), availability.SamplingWindow) {
		return availability.ErrOutsideSamplingWindow
	}

	// empty square => available
	if share.DataHash(hdr.DAH.Hash()).IsEmptyEDS() {
		return nil
	}

	squareSize := len(hdr.DAH.RowRoots)
	samples := light.NewSamplingResult(squareSize, a.sampleCount).Remaining
	for _, s := range samples {
		if err := a.sampleCoord(ctx, hdr, s); err != nil {
			return share.ErrNotAvailable
		}
	}
	return nil
}

func (a *Availability) sampleCoord(ctx context.Context, hdr *header.ExtendedHeader, c shwap.SampleCoords) error {
	row := uint16(c.Row)
	col := uint16(c.Col)

	frags := a.node.Service().GetFragments(hdr.Height(), row, col)
	if len(frags) < a.k {
		return fmt.Errorf("cda: insufficient fragments: have %d, need %d", len(frags), a.k)
	}

	colComs, err := a.node.CommitmentSource().ColumnCommitments(ctx, hdr.Height(), col)
	if err != nil {
		return err
	}

	G := make([][]blsfr.Element, 0, a.k)
	y := make([]blsfr.Element, 0, a.k)

	for _, f := range frags {
		// Re-verify (defense-in-depth; also makes tests simpler).
		// This may fail with nil storage outcome if the fragment was already stored; we only care about validity.
		_, vErr := a.node.Service().ValidateAndStoreFragment(a.node.Verifier(), colComs, f)
		if vErr != nil {
			continue
		}

		cv := share.MaterializeCodingVectorCoeffs(f.CodingVec)
		if int(cv.K) != a.k || len(cv.Coeffs) < a.k {
			continue
		}

		rowG := make([]blsfr.Element, a.k)
		for i := 0; i < a.k; i++ {
			rowG[i].SetBytes(cv.Coeffs[i].Bytes)
		}
		G = append(G, rowG)

		var yi blsfr.Element
		yi.SetBytes(f.Data)
		y = append(y, yi)

		if len(G) == a.k {
			break
		}
	}

	if len(G) < a.k {
		return fmt.Errorf("cda: not enough valid fragments after filtering: have %d, need %d", len(G), a.k)
	}

	_, err = share.SolveLinearSystem(G, y)
	return err
}

var _ share.Availability = (*Availability)(nil)

