package share

import (
	"github.com/celestiaorg/celestia-node/share"
)

func WithBlackBoxMetrics(sg share.Getter) (share.Getter, error) {
	insShareGetter, err := newInstrument(sg)
	if err != nil {
		log.Error("failed to create instrumented share getter", "err", err)
		return nil, err
	}

	return insShareGetter, nil
}
