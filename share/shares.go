package share

import (
	"bytes"
	"fmt"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	types "github.com/celestiaorg/celestia-node/share/pb"
)

type Shares []Share

func (shrs Shares) ToProto() []*types.Share {
	protoShares := make([]*types.Share, len(shrs))
	for i, shr := range shrs {
		protoShares[i] = &types.Share{Data: shr}
	}
	return protoShares
}

func (shrs Shares) Validate(dah *Root, idx int) error {
	if len(shrs) == 0 {
		return fmt.Errorf("empty half row")
	}
	if len(shrs) != len(dah.RowRoots) {
		return fmt.Errorf("shares size doesn't match root size: %d != %d", len(shrs), len(dah.RowRoots))
	}

	return shrs.VerifyRoot(dah, idx)
}

func (shrs Shares) VerifyRoot(dah *Root, idx int) error {
	sqrLn := uint64(len(shrs) / 2)
	tree := wrapper.NewErasuredNamespacedMerkleTree(sqrLn, uint(idx))
	for _, s := range shrs {
		err := tree.Push(s)
		if err != nil {
			return fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	root, err := tree.Root()
	if err != nil {
		return fmt.Errorf("while computing NMT root: %w", err)
	}

	if !bytes.Equal(dah.RowRoots[idx], root) {
		return fmt.Errorf("invalid root hash: %X != %X", root, dah.RowRoots[idx])
	}
	return nil
}

func SharesFromProto(shrs []*types.Share) Shares {
	share := make(Shares, len(shrs))
	for i, shr := range shrs {
		share[i] = ShareFromProto(shr)
	}
	return share
}
