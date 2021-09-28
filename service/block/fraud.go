package block

import core "github.com/celestiaorg/celestia-core/types"

// validateEncoding validates the erasure coding DataRoot in the raw block header against the
// hash of the generated DataAvailabilityHeader, generating a fraud proof if there is a mismatch.
// TODO @renaynay: this method is stubbed out for now.
func (s *Service) validateEncoding(extendedBlock *ExtendedBlock, rawHeader core.Header) error {
	return nil
}
