package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	blsfr "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	cdaavail "github.com/celestiaorg/celestia-node/share/availability/cda"
	"github.com/celestiaorg/celestia-node/share/crypto"
)

// TestCDA_Reconstruct_FromFragments simulates the CDA path at a single coordinate:
// - A header carries column commitments.
// - A node validates+stores k coded fragments (KZG gate).
// - CDA availability reconstructs by solving G*x=y over the field using only fragments+commitments.
//
// This is an incremental E2E test (no merkle proofs, no fraud proofs).
func TestCDA_Reconstruct_FromFragments(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build a minimal header (height=1) and attach CDA commitments.
	hdr := headertest.RandExtendedHeader(t)
	hdr.RawHeader.Height = 1

	verifier := crypto.NewBLS12381HomomorphicKZG()

	const k = 4
	colComs := make([]crypto.ColumnCommitment, 0, k)
	_, _, g1Gen, _ := bls12381.Generators()
	for i := 0; i < k; i++ {
		var p bls12381.G1Affine
		p.ScalarMultiplication(&g1Gen, big.NewInt(int64(i+1)))
		b := p.Marshal()
		colComs = append(colComs, crypto.ColumnCommitment(b))
	}
	hdr.CDAH = &header.CDAHeader{ColumnCommitments: [][]byte{colComs[0], colComs[1], colComs[2], colComs[3]}}

	// Commitment source (test-only; production uses header-backed provider).
	commitSrc := share.StaticColumnCommitments{Commitments: colComs}

	// Stand up a CDANode with pubsub (we won't rely on network delivery; we store directly).
	host, err := libp2p.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = host.Close() })

	ps, err := pubsub.NewGossipSub(ctx, host)
	require.NoError(t, err)

	grid := share.NewRDAGridManager(share.GridDimensions{Rows: 1, Cols: 1})
	node := share.NewCDANode(
		host,
		ps,
		grid,
		verifier,
		commitSrc,
		share.CDANodeConfig{Service: share.CDAServiceConfig{K: k, Buffer: 0}},
	)

	// Simulate k unique peers (peer-id binding policy: one fragment per peer per coordinate).
	peers := make([]peer.ID, k)
	for i := 0; i < k; i++ {
		h, err := libp2p.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = h.Close() })
		peers[i] = h.ID()
	}

	// Create x and a simple full-rank G, compute y = Gx.
	x := make([]blsfr.Element, k)
	for i := 0; i < k; i++ {
		x[i].SetUint64(uint64(11 + i))
	}

	G := make([][]blsfr.Element, k)
	y := make([]blsfr.Element, k)
	for r := 0; r < k; r++ {
		G[r] = make([]blsfr.Element, k)
		for c := 0; c < k; c++ {
			G[r][c].SetUint64(uint64(2 + r + 3*c))
		}
		var acc blsfr.Element
		for c := 0; c < k; c++ {
			var tmp blsfr.Element
			tmp.Mul(&G[r][c], &x[c])
			acc.Add(&acc, &tmp)
		}
		y[r] = acc
	}

	// Store k fragments (with KZG-valid proofs) for coordinate (row=0,col=0).
	for r := 0; r < k; r++ {
		coeffs := make([]crypto.FieldElement, k)
		for c := 0; c < k; c++ {
			b := G[r][c].Bytes()
			coeffs[c] = crypto.FieldElement{Bytes: b[:]}
		}
		cv := crypto.CodingVector{K: k, Coeffs: coeffs}

		proofBytes, err := verifier.CombineCommitments(colComs, cv)
		require.NoError(t, err)

		yb := y[r].Bytes()
		f := share.Fragment{
			ID: share.FragmentID{
				Height: hdr.Height(),
				Row:    0,
				Col:    0,
				NodeID: peers[r],
			},
			CodingVec: cv,
			Data:      yb[:],
			Proof:     crypto.FragmentProof{ProofBytes: proofBytes},
		}

		ok, err := node.Service().ValidateAndStoreFragment(verifier, colComs, f)
		require.NoError(t, err)
		require.True(t, ok)
	}

	avail := cdaavail.New(node, cdaavail.Config{SampleAmount: 1, K: k})
	require.NoError(t, avail.SharesAvailable(ctx, hdr))
}

