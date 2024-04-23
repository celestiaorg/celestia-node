package p2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/network"
)

// RecoveryMiddleware is a middleware that recovers from panics in the handler.
func RecoveryMiddleware(handler network.StreamHandler) network.StreamHandler {
	return func(stream network.Stream) {
		defer func() {
			r := recover()
			if r != nil {
				err := fmt.Errorf("PANIC while handling request: %s", r)
				log.Error(err)
			}
		}()
		handler(stream)
	}
}
