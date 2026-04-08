package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v8/test/util/testnode"
)

// startNetwork creates and starts a test Network with retry logic to handle
// transient "address already in use" failures. These occur because
// MustGetFreePort uses UDP probing, but the actual listeners bind TCP —
// another process can grab the port in the window between the two.
func startNetwork(t *testing.T, cfg *testnode.Config) *Network {
	t.Helper()

	const maxRetries = 3
	var network *Network
	for attempt := range maxRetries {
		if attempt > 0 {
			reallocateTestNodePorts(cfg)
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

// reallocateTestNodePorts assigns fresh free ports to all network addresses in cfg,
// replacing the ones allocated at config-creation time that may now be taken.
func reallocateTestNodePorts(cfg *testnode.Config) {
	cfg.TmConfig.RPC.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.MustGetFreePort())
	cfg.TmConfig.P2P.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.MustGetFreePort())
	cfg.TmConfig.RPC.GRPCListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.MustGetFreePort())
	cfg.AppConfig.GRPC.Address = fmt.Sprintf("127.0.0.1:%d", testnode.MustGetFreePort())
	cfg.AppConfig.API.Address = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.MustGetFreePort())
}
