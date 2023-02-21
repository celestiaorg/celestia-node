package share

import (
	"github.com/celestiaorg/celestia-node/share"
)

// WithMetrics is a share module option that wraps the share getter
// with a proxied version of it that records metrics on
// each share getter method call.
// Use this in a `fx.Decorate` call to replace existing share getter
func WithMetrics(sg share.Getter) (share.Getter, error) {
	insShareGetter, err := newInstrument(sg)
	if err != nil {
		log.Error("failed to create instrumented share getter", "err", err)
		return nil, err
	}

	return insShareGetter, nil
}
