package block

import (
	"bytes"
	"fmt"

	"github.com/celestiaorg/celestia-node/service/header"
)

// validateEncoding validates the erasure coding DataRoot in the raw block header against the
// hash of the generated DataAvailabilityHeader, generating a fraud proof if there is a mismatch.
// TODO @renaynay: this method is stubbed out for now. It only performs a basic validation to check
// whether the hash of the DAH equals the data root in the raw header.
func validateEncoding(block *Block, rawHeader header.RawHeader) error {
	if w, g := block.header.DAH.Hash(), rawHeader.DataHash; !bytes.Equal(w, g) {
		return fmt.Errorf("wrong DataHash in RawHeader. Expected %X, got %X", w, g)
	}
	return nil
}
