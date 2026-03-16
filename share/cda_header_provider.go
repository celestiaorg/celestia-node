package share

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/crypto"
)

// HeaderByHeightGetter is the minimal interface required to fetch headers by height.
// nodebuilder's header service implements this.
type HeaderByHeightGetter interface {
	GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error)
}

// HeaderColumnCommitmentProvider fetches CDA column commitments from ExtendedHeaders.
type HeaderColumnCommitmentProvider struct {
	h HeaderByHeightGetter
}

func NewHeaderColumnCommitmentProvider(h HeaderByHeightGetter) *HeaderColumnCommitmentProvider {
	return &HeaderColumnCommitmentProvider{h: h}
}

func (p *HeaderColumnCommitmentProvider) ColumnCommitments(ctx context.Context, height uint64, col uint16) ([]crypto.ColumnCommitment, error) {
	_ = col // in the current header format we store all commitments; column selection can be added later.
	if p == nil || p.h == nil {
		return nil, fmt.Errorf("cda: nil header provider")
	}
	hdr, err := p.h.GetByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	if hdr.CDAH == nil || len(hdr.CDAH.ColumnCommitments) == 0 {
		return nil, fmt.Errorf("cda: missing CDA header commitments at height %d", height)
	}

	out := make([]crypto.ColumnCommitment, 0, len(hdr.CDAH.ColumnCommitments))
	for _, c := range hdr.CDAH.ColumnCommitments {
		out = append(out, crypto.ColumnCommitment(c))
	}
	return out, nil
}

