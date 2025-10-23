package shwap_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestSampleValidate(t *testing.T) {
	const odsSize = 8
	randEDS := edstest.RandEDS(t, odsSize)
	root, err := share.NewAxisRoots(randEDS)
	require.NoError(t, err)
	inMem := eds.Rsmt2D{ExtendedDataSquare: randEDS}

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for rowIdx := range odsSize * 2 {
			for colIdx := range odsSize * 2 {
				idx := shwap.SampleCoords{Row: rowIdx, Col: colIdx}

				sample, err := inMem.SampleForProofAxis(idx, proofType)
				require.NoError(t, err)

				require.NoError(t, sample.Verify(root, rowIdx, colIdx), "row: %d col: %d", rowIdx, colIdx)
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

	sample, err := inMem.Sample(context.Background(), shwap.SampleCoords{})
	require.NoError(t, err)
	err = sample.Verify(root, 0, 0)
	require.NoError(t, err)

	// incorrect row index
	err = sample.Verify(root, 1, 0)
	require.ErrorIs(t, err, shwap.ErrFailedVerification)

	// Corrupt the share
	b := sample.ToBytes()
	b[0] ^= 0xFF
	shr, err := libshare.NewShare(b)
	require.NoError(t, err)
	sample.Share = *shr
	err = sample.Verify(root, 0, 0)
	require.ErrorIs(t, err, shwap.ErrFailedVerification)

	// incorrect proofType
	sample, err = inMem.Sample(context.Background(), shwap.SampleCoords{})
	require.NoError(t, err)
	sample.ProofType = rsmt2d.Col
	err = sample.Verify(root, 0, 0)
	require.ErrorIs(t, err, shwap.ErrFailedVerification)

	// Corrupt the last root hash byte
	sample, err = inMem.Sample(context.Background(), shwap.SampleCoords{})
	require.NoError(t, err)
	root.RowRoots[0][len(root.RowRoots[0])-1] ^= 0xFF
	err = sample.Verify(root, 0, 0)
	require.ErrorIs(t, err, shwap.ErrFailedVerification)
}

func TestSampleProtoEncoding(t *testing.T) {
	const odsSize = 8
	randEDS := edstest.RandEDS(t, odsSize)
	inMem := eds.Rsmt2D{ExtendedDataSquare: randEDS}

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for rowIdx := range odsSize * 2 {
			for colIdx := range odsSize * 2 {
				idx := shwap.SampleCoords{Row: rowIdx, Col: colIdx}

				sample, err := inMem.SampleForProofAxis(idx, proofType)
				require.NoError(t, err)

				pb := sample.ToProto()
				sampleOut, err := shwap.SampleFromProto(pb)
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

	sample, err := inMem.SampleForProofAxis(shwap.SampleCoords{}, rsmt2d.Row)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sample.Verify(root, 0, 0)
	}
}

func TestSampleJSON(t *testing.T) {
	const odsSize = 8
	randEDS := edstest.RandEDS(t, odsSize)
	inMem := eds.Rsmt2D{ExtendedDataSquare: randEDS}

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for rowIdx := range odsSize * 2 {
			for colIdx := range odsSize * 2 {
				idx := shwap.SampleCoords{Row: rowIdx, Col: colIdx}

				sample, err := inMem.SampleForProofAxis(idx, proofType)
				require.NoError(t, err)

				b, err := sample.MarshalJSON()
				require.NoError(t, err)

				var sampleOut shwap.Sample
				err = sampleOut.UnmarshalJSON(b)
				require.NoError(t, err)

				require.Equal(t, sample, sampleOut)
			}
		}
	}
}
