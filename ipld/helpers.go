package ipld

import (
	"github.com/ipfs/go-cid"

	"fmt"
)

// job represents an encountered node to investigate during the `GetShares` and `GetSharesByNamespace` routines
type job struct {
	id  cid.Cid
	pos int
}

func SanityCheckNID(nID []byte) error {
	if len(nID) != NamespaceSize {
		return fmt.Errorf("expected namespace ID of size %d, got %d", NamespaceSize, len(nID))
	}
	return nil
}
