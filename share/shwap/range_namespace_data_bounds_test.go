package shwap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/nmt"
)

// TestVerifyProofBounds covers each check in verifyProofBounds on its own.
// Only the proof bounds matter here, so the proofs carry no nodes.
func TestVerifyProofBounds(t *testing.T) {
	const odsSize = 4
	proof := func(start, end int) *nmt.Proof {
		p := nmt.NewInclusionProof(start, end, nil, true)
		return &p
	}
	multiFrom, multiTo := SampleCoords{Row: 0, Col: 1}, SampleCoords{Row: 2, Col: 2}
	singleFrom, singleTo := SampleCoords{Row: 1, Col: 1}, SampleCoords{Row: 1, Col: 2}

	tests := []struct {
		name        string
		from, to    SampleCoords
		first, last *nmt.Proof
		wantErr     string
	}{
		{
			name: "multi-row honest bounds",
			from: multiFrom, to: multiTo,
			first: proof(1, odsSize), last: proof(0, 3),
		},
		{
			name: "multi-row full rows need no proofs",
			from: SampleCoords{Row: 0, Col: 0}, to: SampleCoords{Row: 2, Col: odsSize - 1},
		},
		{
			name: "missing first row proof",
			from: multiFrom, to: multiTo,
			last:    proof(0, 3),
			wantErr: "missing first row proof",
		},
		{
			name: "first row proof ends before the row edge",
			from: multiFrom, to: multiTo,
			first: proof(1, 3), last: proof(0, 3),
			wantErr: "first row proof must end at",
		},
		{
			name: "missing last row proof",
			from: multiFrom, to: multiTo,
			first:   proof(1, odsSize),
			wantErr: "missing last row proof",
		},
		{
			name: "last row proof starts after col 0",
			from: multiFrom, to: multiTo,
			first: proof(1, odsSize), last: proof(1, 3),
			wantErr: "last row proof must start at 0",
		},
		{
			name: "single partial row with proof",
			from: singleFrom, to: singleTo,
			first: proof(1, 3),
		},
		{
			name: "single full row needs no proof",
			from: SampleCoords{Row: 1, Col: 0}, to: SampleCoords{Row: 1, Col: odsSize - 1},
		},
		{
			name: "single partial row without proof",
			from: singleFrom, to: singleTo,
			wantErr: "missing proof for partial row range",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rngdata := &RangeNamespaceData{FirstIncompleteRowProof: tc.first, LastIncompleteRowProof: tc.last}
			err := rngdata.verifyProofBounds(tc.from, tc.to, odsSize)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
