package ipld

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
)

// job represents an encountered node to investigate during the `GetShares` and `GetSharesByNamespace` routines.
type job struct {
	id  cid.Cid
	pos int
	ctx context.Context
}

func SanityCheckNID(nID []byte) error {
	if len(nID) != NamespaceSize {
		return fmt.Errorf("expected namespace ID of size %d, got %d", NamespaceSize, len(nID))
	}
	return nil
}
