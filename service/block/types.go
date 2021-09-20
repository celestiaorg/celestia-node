package block

import (
	core "github.com/celestiaorg/celestia-core/types"
)

// Raw is an alias to a "raw" Core block. It is "raw" because
// it is still awaiting erasure coding.
type Raw = core.Block
