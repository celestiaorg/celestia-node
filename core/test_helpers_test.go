package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v10/test/util/testnode"

	"github.com/celestiaorg/celestia-node/libs/testnodeutil"
)

// startNetwork creates and starts a test Network with retry logic to handle
// transient "address already in use" failures. Ports are picked before the
// listeners bind them, so another process can take a port in between.
func startNetwork(t *testing.T, cfg *testnode.Config) *Network {
	t.Helper()

	const maxRetries = 3
	var network *Network
	for attempt := range maxRetries {
		if attempt > 0 {
			testnodeutil.AssignFreePorts(cfg)
		}
		network = NewNetwork(t, cfg)
		if err := network.Start(); err != nil {
			if attempt < maxRetries-1 && isAddressInUseError(err) {
				t.Logf("port conflict on attempt %d, retrying with new ports", attempt+1)
				_ = network.Stop()
				continue
			}
			require.NoError(t, err)
		}
		break
	}
	return network
}

// isAddressInUseError reports whether the error is a "bind: address already in use" error.
func isAddressInUseError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "address already in use")
}
