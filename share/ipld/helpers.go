package ipld

import (
	"fmt"
)

func SanityCheckNID(nID []byte) error {
	if len(nID) != NamespaceSize {
		return fmt.Errorf("expected namespace ID of size %d, got %d", NamespaceSize, len(nID))
	}
	return nil
}
