package shwap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestNewSampleFromEDS(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
			for colIdx := 0; colIdx < odsSize*2; colIdx++ {
				sample, err := SampleFromEDS(eds, proofType, rowIdx, colIdx)
				require.NoError(t, err)

				want := eds.GetCell(uint(rowIdx), uint(colIdx))
				require.Equal(t, want, sample.Share)
				require.Equal(t, proofType, sample.ProofType)
				require.NotNil(t, sample.Proof)
				require.Equal(t, sample.Proof.End()-sample.Proof.Start(), 1)
				require.Len(t, sample.Proof.Nodes(), 4)
			}
		}
	}
}

func TestSampleValidate(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
			for colIdx := 0; colIdx < odsSize*2; colIdx++ {
				sample, err := SampleFromEDS(eds, proofType, rowIdx, colIdx)
				require.NoError(t, err)

				require.True(t, sample.verifyInclusion(root, rowIdx, colIdx))
				require.NoError(t, sample.Validate(root, rowIdx, colIdx))
			}
		}
	}
}

// TestSampleNegativeVerifyInclusion checks
func TestSampleNegativeVerifyInclusion(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)

	sample, err := SampleFromEDS(eds, rsmt2d.Row, 0, 0)
	require.NoError(t, err)
	included := sample.verifyInclusion(root, 0, 0)
	require.True(t, included)

	// incorrect row index
	included = sample.verifyInclusion(root, 1, 0)
	require.False(t, included)

	// incorrect col index is not used in the inclusion proof verification
	included = sample.verifyInclusion(root, 0, 1)
	require.True(t, included)

	// Corrupt the share
	sample.Share[0] ^= 0xFF
	included = sample.verifyInclusion(root, 0, 0)
	require.False(t, included)

	// incorrect proofType
	sample, err = SampleFromEDS(eds, rsmt2d.Row, 0, 0)
	require.NoError(t, err)
	sample.ProofType = rsmt2d.Col
	included = sample.verifyInclusion(root, 0, 0)
	require.False(t, included)

	// Corrupt the last root hash byte
	sample, err = SampleFromEDS(eds, rsmt2d.Row, 0, 0)
	require.NoError(t, err)
	root.RowRoots[0][len(root.RowRoots[0])-1] ^= 0xFF
	included = sample.verifyInclusion(root, 0, 0)
	require.False(t, included)
}

func TestSampleProtoEncoding(t *testing.T) {
	const odsSize = 8
	eds := edstest.RandEDS(t, odsSize)

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
			for colIdx := 0; colIdx < odsSize*2; colIdx++ {
				sample, err := SampleFromEDS(eds, proofType, rowIdx, colIdx)
				require.NoError(t, err)

				pb := sample.ToProto()
				sampleOut := SampleFromProto(pb)
				require.NoError(t, err)
				require.Equal(t, sample, sampleOut)
			}
		}
	}
}

// BenchmarkSampleValidate benchmarks the performance of sample validation.
// BenchmarkSampleValidate-10    	  284829	      3935 ns/op
func BenchmarkSampleValidate(b *testing.B) {
	const odsSize = 32
	eds := edstest.RandEDS(b, odsSize)
	root, err := share.NewRoot(eds)
	require.NoError(b, err)
	sample, err := SampleFromEDS(eds, rsmt2d.Row, 0, 0)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sample.Validate(root, 0, 0)
	}
}
