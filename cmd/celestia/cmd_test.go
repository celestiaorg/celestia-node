package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLight(t *testing.T) {
	// Run the tests in a temporary directory
	tmpDir, err := ioutil.TempDir("", "light")
	require.NoError(t, err, "error creating a temporary test directory")
	testDir, err := os.Getwd()
	require.NoError(t, err, "error getting the current working directory")
	err = os.Chdir(tmpDir)
	require.NoError(t, err, "error changing to the temporary test directory")

	t.Run("init", func(t *testing.T) {
		output := &bytes.Buffer{}
		rootCmd.SetOut(output)
		rootCmd.SetArgs([]string{
			"bridge",
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

func TestBridge(t *testing.T) {
	// Run the tests in a temporary directory
	tmpDir, err := ioutil.TempDir("", "bridge")
	require.NoError(t, err, "error creating a temporary test directory")
	testDir, err := os.Getwd()
	require.NoError(t, err, "error getting the current working directory")
	err = os.Chdir(tmpDir)
	require.NoError(t, err, "error changing to the temporary test directory")

	t.Run("init", func(t *testing.T) {
		output := &bytes.Buffer{}
		rootCmd.SetOut(output)
		rootCmd.SetArgs([]string{
			"bridge",
			"--node.store", ".celestia-bridge",
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
