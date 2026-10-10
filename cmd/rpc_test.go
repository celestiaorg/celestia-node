package cmd

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	nodemod "github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestInitClientLoadsLocalURLWithExplicitToken(t *testing.T) {
	previousURL := requestURL
	previousToken := authTokenFlag
	previousTimeout := timeoutFlag
	t.Cleanup(func() {
		requestURL = previousURL
		authTokenFlag = previousToken
		timeoutFlag = previousTimeout
	})

	storePath := t.TempDir()
	cfg := nodebuilder.DefaultConfig(nodemod.Light)
	cfg.RPC.Address = "127.0.0.2"
	cfg.RPC.Port = "12345"
	require.NoError(t, nodebuilder.SaveConfig(filepath.Join(storePath, "config.toml"), cfg))

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	cmd.Flags().AddFlagSet(RPCFlags())
	require.NoError(t, cmd.Flags().Set(nodeStoreFlag, storePath))
	require.NoError(t, cmd.Flags().Set("token", "token"))

	require.NoError(t, InitClient(cmd, nil))
	client, err := ParseClientFromCtx(cmd.Context())
	require.NoError(t, err)
	client.Close()
	require.Equal(t, cfg.RPC.RequestURL(), requestURL)
	require.Equal(t, "token", authTokenFlag)
}
