package file

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestValidatingFile_Share(t *testing.T) {
	tests := []struct {
		name       string
		x, y       int
		odsSize    int
		expectFail bool
	}{
		{"ValidIndices", 3, 2, 4, false},
		{"OutOfBoundsX", 8, 3, 4, true},
		{"OutOfBoundsY", 3, 8, 4, true},
		{"NegativeX", -1, 4, 6, true},
		{"NegativeY", 3, -1, 6, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eds := edstest.RandEDS(t, tt.odsSize)
			file := &MemFile{Eds: eds}
			vf := NewValidatingFile(file)

			_, err := vf.Share(context.Background(), tt.x, tt.y)
			if tt.expectFail {
				require.ErrorIs(t, err, ErrOutOfBounds)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidatingFile_AxisHalf(t *testing.T) {
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
			eds := edstest.RandEDS(t, tt.odsSize)
			file := &MemFile{Eds: eds}
			vf := NewValidatingFile(file)

			_, err := vf.AxisHalf(context.Background(), tt.axisType, tt.axisIdx)
			if tt.expectFail {
				require.ErrorIs(t, err, ErrOutOfBounds)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidatingFile_Data(t *testing.T) {
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
			eds := edstest.RandEDS(t, tt.odsSize)
			file := &MemFile{Eds: eds}
			vf := NewValidatingFile(file)

			ns := sharetest.RandV0Namespace()
			_, err := vf.Data(context.Background(), ns, tt.rowIdx)
			if tt.expectFail {
				require.ErrorIs(t, err, ErrOutOfBounds)
			} else {
				require.True(t, err == nil || errors.Is(err, ipld.ErrNamespaceOutsideRange))
			}
		})
	}
}
