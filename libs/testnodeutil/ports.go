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
// testnode.DefaultAppConfig.
func AssignFreePorts(cfg *testnode.Config) {
	cfg.TmConfig.RPC.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", FreePort())
	cfg.TmConfig.P2P.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", FreePort())
	cfg.TmConfig.RPC.GRPCListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", FreePort())
	cfg.TmConfig.PrivValidatorGRPCListenAddr = fmt.Sprintf("127.0.0.1:%d", FreePort())
	cfg.AppConfig.GRPC.Address = fmt.Sprintf("127.0.0.1:%d", FreePort())
	cfg.AppConfig.API.Address = fmt.Sprintf("tcp://127.0.0.1:%d", FreePort())
}

// FreePort returns a TCP port on 127.0.0.1 that the kernel picked and that was
// free when FreePort returned. It panics if no port can be bound.
func FreePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("testnodeutil: binding a free port: %v", err))
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
