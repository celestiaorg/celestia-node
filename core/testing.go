package core

import (
	"fmt"
	"net"
	"net/url"
	"testing"

	appconfig "github.com/cosmos/cosmos-sdk/server/config"
	"github.com/stretchr/testify/require"
	tmconfig "github.com/tendermint/tendermint/config"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/celestiaorg/celestia-app/testutil/testnode"
)

// TestConfig encompasses all the configs required to run test Tendermint + Celestia App tandem.
type TestConfig struct {
	ConsensusParams *tmproto.ConsensusParams
	Tendermint      *tmconfig.Config
	App             *appconfig.Config

	Accounts     []string
	SuppressLogs bool
}

// DefaultTestConfig returns the default testing configuration for Tendermint + Celestia App tandem.
//
// It fetches free ports from OS and sets them into configs, s.t.
// user can make use of them(unlike 0 port) and allowing to run
// multiple tests nodes in parallel.
//
// Additionally, it instructs Tendermint + Celestia App tandem to setup 10 funded accounts.
func DefaultTestConfig() *TestConfig {
	conCfg := testnode.DefaultParams()

	tnCfg := testnode.DefaultTendermintConfig()
	tnCfg.RPC.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", getFreePort())
	tnCfg.RPC.GRPCListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", getFreePort())
	tnCfg.P2P.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", getFreePort())

	appCfg := testnode.DefaultAppConfig()
	appCfg.GRPC.Address = fmt.Sprintf("127.0.0.1:%d", getFreePort())
	appCfg.API.Address = fmt.Sprintf("tcp://127.0.0.1:%d", getFreePort())

	// instructs creating funded accounts
	// 10 usually is enough for testing
	accounts := make([]string, 10)
	for i := range accounts {
		accounts[i] = tmrand.Str(9)
	}

	return &TestConfig{
		ConsensusParams: conCfg,
		Tendermint:      tnCfg,
		App:             appCfg,
		Accounts:        accounts,
		SuppressLogs:    true,
	}
}

// StartTestNode simply starts Tendermint and Celestia App tandem with default testing
// configuration.
func StartTestNode(t *testing.T) testnode.Context {
	return StartTestNodeWithConfig(t, DefaultTestConfig())
}

// StartTestNodeWithConfig starts Tendermint and Celestia App tandem with custom configuration.
func StartTestNodeWithConfig(t *testing.T, cfg *TestConfig) testnode.Context {
	state, kr, err := testnode.DefaultGenesisState(cfg.Accounts...)
	require.NoError(t, err)

	tmNode, app, cctx, err := testnode.New(
		t,
		cfg.ConsensusParams,
		cfg.Tendermint,
		cfg.SuppressLogs,
		state,
		kr,
		"private",
		nil,
	)
	require.NoError(t, err)

	cctx, cleanupCoreNode, err := testnode.StartNode(tmNode, cctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := cleanupCoreNode()
		require.NoError(t, err)
	})

	cctx, cleanupGRPCServer, err := StartGRPCServer(app, cfg.App, cctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := cleanupGRPCServer()
		require.NoError(t, err)
	})

	// we want to test over remote http client,
	// so we are as close to the real environment as possible
	// however, it might be useful to use local tendermint client
	// if you need to debug something inside of it
	ip, port, err := getEndpoint(cfg.Tendermint)
	require.NoError(t, err)
	client, err := NewRemote(ip, port)
	require.NoError(t, err)

	err = client.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		err := client.Stop()
		require.NoError(t, err)
	})

	cctx.WithClient(client)
	return cctx
}

func getFreePort() int {
	a, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port
		}
	}
	panic("while getting free port: " + err.Error())
}

func getEndpoint(cfg *tmconfig.Config) (string, string, error) {
	url, err := url.Parse(cfg.RPC.ListenAddress)
	if err != nil {
		return "", "", err
	}
	host, _, err := net.SplitHostPort(url.Host)
	if err != nil {
		return "", "", err
	}
	return host, url.Port(), nil
}
