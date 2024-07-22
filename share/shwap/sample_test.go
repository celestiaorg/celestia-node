package shwap_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestSampleValidate(t *testing.T) {
	const odsSize = 8
	randEDS := edstest.RandEDS(t, odsSize)
	root, err := share.NewAxisRoots(randEDS)
	require.NoError(t, err)
	inMem := eds.Rsmt2D{ExtendedDataSquare: randEDS}

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
			for colIdx := 0; colIdx < odsSize*2; colIdx++ {
				sample, err := inMem.SampleForProofAxis(rowIdx, colIdx, proofType)
				require.NoError(t, err)

				require.NoError(t, sample.Validate(root, rowIdx, colIdx))
			}
		}
	}
}

// TestSampleNegativeVerifyInclusion checks
func TestSampleNegativeVerifyInclusion(t *testing.T) {
	const odsSize = 8
	randEDS := edstest.RandEDS(t, odsSize)
	root, err := share.NewAxisRoots(randEDS)
	require.NoError(t, err)
	inMem := eds.Rsmt2D{ExtendedDataSquare: randEDS}

	sample, err := inMem.Sample(context.Background(), 0, 0)
	require.NoError(t, err)
	err = sample.Validate(root, 0, 0)
	require.NoError(t, err)

	// incorrect row index
	err = sample.Validate(root, 1, 0)
	require.ErrorIs(t, err, shwap.ErrFailedVerification)

	// Corrupt the share
	sample.Share[0] ^= 0xFF
	err = sample.Validate(root, 0, 0)
	require.ErrorIs(t, err, shwap.ErrFailedVerification)

	// incorrect proofType
	sample, err = inMem.Sample(context.Background(), 0, 0)
	require.NoError(t, err)
	sample.ProofType = rsmt2d.Col
	err = sample.Validate(root, 0, 0)
	require.ErrorIs(t, err, shwap.ErrFailedVerification)

	// Corrupt the last root hash byte
	sample, err = inMem.Sample(context.Background(), 0, 0)
	require.NoError(t, err)
	root.RowRoots[0][len(root.RowRoots[0])-1] ^= 0xFF
	err = sample.Validate(root, 0, 0)
	require.ErrorIs(t, err, shwap.ErrFailedVerification)
}

func TestSampleProtoEncoding(t *testing.T) {
	const odsSize = 8
	randEDS := edstest.RandEDS(t, odsSize)
	inMem := eds.Rsmt2D{ExtendedDataSquare: randEDS}

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for rowIdx := 0; rowIdx < odsSize*2; rowIdx++ {
			for colIdx := 0; colIdx < odsSize*2; colIdx++ {
				sample, err := inMem.SampleForProofAxis(rowIdx, colIdx, proofType)
				require.NoError(t, err)

				pb := sample.ToProto()
				sampleOut := shwap.SampleFromProto(pb)
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
	randEDS := edstest.RandEDS(b, odsSize)
	root, err := share.NewAxisRoots(randEDS)
	require.NoError(b, err)
	inMem := eds.Rsmt2D{ExtendedDataSquare: randEDS}
	sample, err := inMem.SampleForProofAxis(0, 0, rsmt2d.Row)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sample.Validate(root, 0, 0)
	}
}
