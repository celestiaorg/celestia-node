package block

import (
	"github.com/celestiaorg/celestia-node/service/header"
)

// validateEncoding validates the erasure coding DataRoot in the raw block header against the
// hash of the generated DataAvailabilityHeader, generating a fraud proof if there is a mismatch.
// TODO @renaynay: this method is stubbed out for now.
func (s *Service) validateEncoding(extendedBlock *Block, rawHeader header.Raw) error {
	return nil
}
