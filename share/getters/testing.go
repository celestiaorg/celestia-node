package getters

import (
	"testing"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/share"
)

func TestGetter(t *testing.T) (share.Getter, *share.Root) {
	eds := share.RandEDS(t, 8)
	dah := da.NewDataAvailabilityHeader(eds)
	return &SingleEDSGetter{
		EDS: eds,
	}, &dah
}
