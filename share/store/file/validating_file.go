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

// ValidatingFile is a file implementation that performs sanity checks on file operations.
type ValidatingFile struct {
	EdsFile
}

func NewValidatingFile(f EdsFile) EdsFile {
	return &ValidatingFile{EdsFile: f}
}

func (f *ValidatingFile) Share(ctx context.Context, x, y int) (*share.ShareWithProof, error) {
	if err := validateIndexBounds(f, x); err != nil {
		return nil, fmt.Errorf("col: %w", err)
	}
	if err := validateIndexBounds(f, y); err != nil {
		return nil, fmt.Errorf("row: %w", err)
	}
	return f.EdsFile.Share(ctx, x, y)
}

func (f *ValidatingFile) AxisHalf(ctx context.Context, axisType rsmt2d.Axis, axisIdx int) ([]share.Share, error) {
	if err := validateIndexBounds(f, axisIdx); err != nil {
		return nil, fmt.Errorf("%s: %w", axisType, err)
	}
	return f.EdsFile.AxisHalf(ctx, axisType, axisIdx)
}

func (f *ValidatingFile) Data(ctx context.Context, namespace share.Namespace, rowIdx int) (share.NamespacedRow, error) {
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
