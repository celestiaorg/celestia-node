package store

import (
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/utils"
)

// Deprecated: Use utils.StripLock instead
type striplock = utils.StripLock

// Deprecated: Use utils.MultiLock instead
type multiLock = utils.MultiLock

// Deprecated: Use utils.NewStripLock instead
func newStripLock(size int) *striplock {
	return utils.NewStripLock(size)
}
