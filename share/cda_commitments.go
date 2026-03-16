package share

import (
	"context"

	"github.com/celestiaorg/celestia-node/share/crypto"
)

// StaticColumnCommitments is a minimal commitment provider used to unblock
// CDA pubsub/storage integration before ExtendedHeader is updated to carry
// real KZG commitments.
//
// It returns the same commitments for any height/col.
type StaticColumnCommitments struct {
	Commitments []crypto.ColumnCommitment
}

func (s StaticColumnCommitments) ColumnCommitments(ctx context.Context, height uint64, col uint16) ([]crypto.ColumnCommitment, error) {
	_ = ctx
	_ = height
	_ = col
	return s.Commitments, nil
}

