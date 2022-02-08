package state

import (
	"context"
	"fmt"
	"os"
	"testing"

	lens "github.com/strangelove-ventures/lens/client"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
)

func TestCoreAccessor_CurrentBalance(t *testing.T) {
	// construct a chain client
	cc := constructChainClient(t)

	// construct a core accessor
	accessor := NewCoreAccessor(cc)
	bal, err := accessor.CurrentBalance(context.Background())
	require.NoError(t, err)
	t.Log("BAL: ", bal)
}

func constructChainClient(t *testing.T) *lens.ChainClient {
	// start remote core node
	nd, _, endpoint := core.StartRemoteCore()
	t.Cleanup(func() {
		//nolint:errcheck
		nd.Stop()
	})

	tmpDir := t.TempDir()
	conf := &lens.ChainClientConfig{
		Key:            "default",
		ChainID:        "",
		RPCAddr:        fmt.Sprintf("http://%s", endpoint),
		AccountPrefix:  "celes",
		KeyringBackend: "test",
		GasAdjustment:  1.2,
		GasPrices:      "0.01uosmo",
		KeyDirectory:   tmpDir,
		Debug:          true,
		Timeout:        "20s",
		OutputFormat:   "json",
		SignModeStr:    "direct",
		Modules:        lens.ModuleBasics,
	}
	//	conf := &lens.ChainClientConfig{
	//		ChainID:        "celestia",
	//		AccountPrefix:  "celes",
	//		Key:            "default",
	//		KeyringBackend: keyring.BackendTest,
	//		KeyDirectory:   tmpDir,
	//		Debug:          false,
	//		RPCAddr:        fmt.Sprintf("http://%s", endpoint),
	//	}

	cc, err := lens.NewChainClient(conf, tmpDir, os.Stdin, os.Stderr)
	require.NoError(t, err)

	err = cc.Init()
	require.NoError(t, err)

	return cc
}
