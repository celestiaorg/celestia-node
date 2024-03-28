package file

import (
	"context"
	"errors"
	"fmt"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

// ErrOutOfBounds is returned whenever an index is out of bounds.
var ErrOutOfBounds = errors.New("index is out of bounds")

// validatingFile is a file implementation that performs sanity checks on file operations.
type validatingFile struct {
	EdsFile
}

func WithValidation(f EdsFile) EdsFile {
	return &validatingFile{EdsFile: f}
}

func (f *validatingFile) Share(ctx context.Context, x, y int) (*share.ShareWithProof, error) {
	if err := validateIndexBounds(f, x); err != nil {
		return nil, fmt.Errorf("col: %w", err)
	}
	if err := validateIndexBounds(f, y); err != nil {
		return nil, fmt.Errorf("row: %w", err)
	}
	return f.EdsFile.Share(ctx, x, y)
}

func (f *validatingFile) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) (AxisHalf, error) {
	if err := validateIndexBounds(f, axisIdx); err != nil {
		return AxisHalf{}, fmt.Errorf("%s: %w", axisType, err)
	}
	return f.EdsFile.AxisHalf(ctx, axisType, axisIdx)
}

func (f *validatingFile) Data(ctx context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error) {
	if err := validateIndexBounds(f, rowIdx); err != nil {
		return share.NamespacedRow{}, fmt.Errorf("row: %w", err)
	}
	return f.EdsFile.Data(ctx, namespace, rowIdx)
}

// validateIndexBounds checks if the index is within the bounds of the file.
func validateIndexBounds(f EdsFile, idx int) error {
	if idx < 0 || idx >= f.Size() {
		return fmt.Errorf("%w: index %d is out of bounds: [0, %d)", ErrOutOfBounds, idx, f.Size())
	}
	return nil
}
