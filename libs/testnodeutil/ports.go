// Package testnodeutil holds test helpers for running celestia-app test nodes
// (celestia-app/test/util/testnode) inside celestia-node tests.
package testnodeutil

import (
	"fmt"
	"net"

	"github.com/celestiaorg/celestia-app/v10/test/util/testnode"
)

// DefaultConfig returns testnode.DefaultConfig with every listener moved to a
// port picked by the kernel.
//
// testnode.DefaultConfig takes its ports from a counter that starts at 20000 in
// every process. `go test` runs packages as separate processes in parallel, so
// two packages starting test nodes at the same time get the same ports and
// fail with "bind: address already in use". Ports picked by the kernel are
// randomized, so separate processes almost never get the same one.
func DefaultConfig() *testnode.Config {
	cfg := testnode.DefaultConfig()
	AssignFreePorts(cfg)
	return cfg
}

// AssignFreePorts sets every listen address in cfg to a free port picked by
// the kernel. It covers every port set by testnode.DefaultTendermintConfig and
// testnode.DefaultAppConfig. The ports are distinct from each other.
func AssignFreePorts(cfg *testnode.Config) {
	ports := FreePorts(6)
	cfg.TmConfig.RPC.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", ports[0])
	cfg.TmConfig.P2P.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", ports[1])
	cfg.TmConfig.RPC.GRPCListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", ports[2])
	cfg.TmConfig.PrivValidatorGRPCListenAddr = fmt.Sprintf("127.0.0.1:%d", ports[3])
	cfg.AppConfig.GRPC.Address = fmt.Sprintf("127.0.0.1:%d", ports[4])
	cfg.AppConfig.API.Address = fmt.Sprintf("tcp://127.0.0.1:%d", ports[5])
}

// FreePorts returns n distinct TCP ports on 127.0.0.1 that the kernel picked
// and that were free when FreePorts returned. It holds every listener open
// until all n ports are picked, so the kernel can't hand out the same port
// twice. It panics if a port can't be bound.
func FreePorts(n int) []int {
	ports := make([]int, 0, n)
	listeners := make([]net.Listener, 0, n)
	defer func() {
		for _, l := range listeners {
			_ = l.Close()
		}
	}()
	for range n {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			panic(fmt.Sprintf("testnodeutil: binding a free port: %v", err))
		}
		listeners = append(listeners, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	return ports
}
