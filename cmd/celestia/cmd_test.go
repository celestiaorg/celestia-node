package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestLight(t *testing.T) {
	// Run the tests in a temporary directory
	tmpDir := t.TempDir()
	testDir, err := os.Getwd()
	require.NoError(t, err, "error getting the current working directory")
	err = os.Chdir(tmpDir)
	require.NoError(t, err, "error changing to the temporary test directory")

	t.Run("init", func(t *testing.T) {
		output := &bytes.Buffer{}
		rootCmd.SetOut(output)
		rootCmd.SetArgs([]string{
			"light",
			"config",
			"--node.store", ".celestia-light",
			"init",
		})
		err := rootCmd.ExecuteContext(context.Background())
		require.NoError(t, err)
	})

	t.Cleanup(func() {
		if err := os.Chdir(testDir); err != nil {
			t.Error("error resetting:", err)
		}
	})

	// TODO @jbowen93: Commented out until a dry-run option can be implemented
	/*
			t.Run("start", func(t *testing.T) {
				output := &bytes.Buffer{}
				rootCmd.SetOut(output)
				rootCmd.SetArgs([]string{
					"light",
					"--node.store", ".celestia-light",
					"start",
					"--headers.trusted-peer",
		            "/ip4/192.167.10.6/tcp/2121/p2p/12D3KooWL8z3KARAYJcmExhDsGwKbjChKeGaJpFPENyADdxmEHzw",
		            "--headers.trusted-hash",
		            "54A8B66D2BEF13850D67C8D474E196BD7485FE5A79989E31B17169371B0A9C96",
				})
				err := rootCmd.ExecuteContext(cmdnode.WithEnv(context.Background()))
				require.NoError(t, err)
			})
	*/
}

func TestFull(t *testing.T) {
	// Run the tests in a temporary directory
	tmpDir := t.TempDir()
	testDir, err := os.Getwd()
	require.NoError(t, err, "error getting the current working directory")
	err = os.Chdir(tmpDir)
	require.NoError(t, err, "error changing to the temporary test directory")

	t.Run("init", func(t *testing.T) {
		output := &bytes.Buffer{}
		rootCmd.SetOut(output)
		rootCmd.SetArgs([]string{
			"full",
			"config",
			"--node.store", ".celestia-full",
			"init",
		})
		err := rootCmd.ExecuteContext(context.Background())
		require.NoError(t, err)
	})

	t.Cleanup(func() {
		if err := os.Chdir(testDir); err != nil {
			t.Error("error resetting:", err)
		}
	})

	// TODO @jbowen93: Commented out until a dry-run option can be implemented
	/*
			t.Run("start", func(t *testing.T) {
				output := &bytes.Buffer{}
				rootCmd.SetOut(output)
				rootCmd.SetArgs([]string{
					"full",
					"--node.store", ".celestia-full",
					"start",
					"--headers.trusted-peer",
		            "/ip4/192.167.10.6/tcp/2121/p2p/12D3KooWL8z3KARAYJcmExhDsGwKbjChKeGaJpFPENyADdxmEHzw",
		            "--headers.trusted-hash",
		            "54A8B66D2BEF13850D67C8D474E196BD7485FE5A79989E31B17169371B0A9C96",
				})
				err := rootCmd.ExecuteContext(cmdnode.WithEnv(context.Background()))
				require.NoError(t, err)
			})
	*/
}

func TestBridge(t *testing.T) {
	// Run the tests in a temporary directory
	tmpDir := t.TempDir()
	testDir, err := os.Getwd()
	require.NoError(t, err, "error getting the current working directory")
	err = os.Chdir(tmpDir)
	require.NoError(t, err, "error changing to the temporary test directory")

	t.Run("init", func(t *testing.T) {
		output := &bytes.Buffer{}
		rootCmd.SetOut(output)
		rootCmd.SetArgs([]string{
			"bridge",
			"config",
			"--node.store", ".celestia-bridge",
			"init",
		})
		err := rootCmd.ExecuteContext(context.Background())
		require.NoError(t, err)
	})

	t.Run("remove", func(t *testing.T) {
		output := &bytes.Buffer{}
		rootCmd.SetOut(output)

		rootCmd.SetArgs([]string{
			"bridge",
			"config",
			"--node.store", ".celestia-bridge",
			"init",
		})
		err = rootCmd.ExecuteContext(context.Background())

		rootCmd.SetArgs([]string{
			"bridge",
			"config",
			"--node.store", ".celestia-bridge",
			"remove",
		})
		err = rootCmd.ExecuteContext(context.Background())
		require.NoError(t, err)

		// file does not exist
		rootCmd.SetArgs([]string{
			"bridge",
			"config",
			"--node.store", ".celestia-bridge",
			"remove",
		})

		err = rootCmd.ExecuteContext(context.Background())
		require.Error(t, err)
	})

	t.Run("reinit", func(t *testing.T) {
		tempDir, err := os.MkdirTemp(".", "")
		require.NoError(t, err)

		defaultPath := fmt.Sprintf("%s/config.toml", tempDir)
		diffPath := fmt.Sprintf("%s/diffconfig.toml", tempDir)

		diffConfig := nodebuilder.DefaultConfig(node.Bridge)
		require.NoError(t, err)

		// make changes to default config
		diffConfig.State.KeyringAccName = "diffConfig"
		err = nodebuilder.SaveConfig(diffPath, diffConfig)
		require.NoError(t, err)

		output := &bytes.Buffer{}
		rootCmd.SetOut(output)

		// init config
		rootCmd.SetArgs([]string{
			"bridge",
			"config",
			"--node.store", tempDir,
			"init",
		})

		err = rootCmd.ExecuteContext(context.Background())

		config, err := nodebuilder.LoadConfig(defaultPath)
		require.NotEqualValues(t, config, diffConfig)

		// reinit with diff config
		rootCmd.SetArgs([]string{
			"bridge",
			"config",
			"--node.store", tempDir,
			"reinit",
			diffPath,
		})
		err = rootCmd.ExecuteContext(context.Background())
		require.NoError(t, err)

		// load default config and check for changes
		config, err = nodebuilder.LoadConfig(defaultPath)
		require.EqualValues(t, config, diffConfig)
	})

	t.Cleanup(func() {
		if err := os.Chdir(testDir); err != nil {
			t.Error("error resetting:", err)
		}
	})

	// TODO @jbowen93: Commented out until a dry-run option can be implemented
	/*
			t.Run("start", func(t *testing.T) {
				output := &bytes.Buffer{}
				rootCmd.SetOut(output)
				rootCmd.SetArgs([]string{
					"bridge",
					"--node.store", ".celestia-bridge",
					"start",
					"--core.remote",
		            "tcp://192.167.10.2:26657",
					"--headers.trusted-hash",
					"54A8B66D2BEF13850D67C8D474E196BD7485FE5A79989E31B17169371B0A9C96",
				})
				err := rootCmd.ExecuteContext(cmdnode.WithEnv(context.Background()))
				require.NoError(t, err)
			})
	*/
}
