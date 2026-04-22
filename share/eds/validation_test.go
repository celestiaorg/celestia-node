package eds

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestValidation_Sample(t *testing.T) {
	tests := []struct {
		name           string
		rowIdx, colIdx int
		odsSize        int
		expectFail     bool
	}{
		{"ValidIndices", 3, 2, 4, false},
		{"OutOfBoundsX", 8, 3, 4, true},
		{"OutOfBoundsY", 3, 8, 4, true},
		{"NegativeX", -1, 4, 8, true},
		{"NegativeY", 3, -1, 8, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			randEDS := edstest.RandEDS(t, tt.odsSize)
			accessor := &Rsmt2D{ExtendedDataSquare: randEDS}
			validation := WithValidation(AccessorAndStreamer(accessor, nil))

			idx := shwap.SampleCoords{Row: tt.rowIdx, Col: tt.colIdx}

			_, err := validation.Sample(context.Background(), idx)
			if tt.expectFail {
				require.ErrorIs(t, err, shwap.ErrInvalidID)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidation_AxisHalf(t *testing.T) {
	tests := []struct {
		name       string
		axisType   rsmt2d.Axis
		axisIdx    int
		odsSize    int
		expectFail bool
	}{
		{"ValidIndex", rsmt2d.Row, 2, 4, false},
		{"OutOfBounds", rsmt2d.Col, 8, 4, true},
		{"NegativeIndex", rsmt2d.Row, -1, 4, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			randEDS := edstest.RandEDS(t, tt.odsSize)
			accessor := &Rsmt2D{ExtendedDataSquare: randEDS}
			validation := WithValidation(AccessorAndStreamer(accessor, nil))

			_, err := validation.AxisHalf(context.Background(), tt.axisType, tt.axisIdx)
			if tt.expectFail {
				require.ErrorIs(t, err, shwap.ErrInvalidID)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidation_RowNamespaceData(t *testing.T) {
	tests := []struct {
		name       string
		rowIdx     int
		odsSize    int
		expectFail bool
	}{
		{"ValidIndex", 3, 4, false},
		{"OutOfBounds", 8, 4, true},
		{"NegativeIndex", -1, 4, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			randEDS := edstest.RandEDS(t, tt.odsSize)
			accessor := &Rsmt2D{ExtendedDataSquare: randEDS}
			validation := WithValidation(AccessorAndStreamer(accessor, nil))

			ns := libshare.RandomNamespace()
			_, err := validation.RowNamespaceData(context.Background(), ns, tt.rowIdx)
			if tt.expectFail {
				require.ErrorIs(t, err, shwap.ErrInvalidID)
			} else {
				require.True(t, err == nil || errors.Is(err, shwap.ErrNamespaceOutsideRange), err)
			}
		})
	}
}
