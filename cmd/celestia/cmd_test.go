package main

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
)

func TestCompletionHelpString(t *testing.T) {
	type TestFields struct {
		NoInputOneOutput        func(context.Context) (*header.ExtendedHeader, error)
		TwoInputsOneOutputArray func(
			context.Context,
			*header.ExtendedHeader,
			uint64,
		) ([]*header.ExtendedHeader, error)
		OneInputOneOutput  func(context.Context, uint64) (*header.ExtendedHeader, error)
		NoInputsNoOutputs  func(ctx context.Context) error
		NoInputsChanOutput func(ctx context.Context) (<-chan *header.ExtendedHeader, error)
	}
	testOutputs := []string{
		"() -> (*header.ExtendedHeader)",
		"(*header.ExtendedHeader, uint64) -> ([]*header.ExtendedHeader)",
		"(uint64) -> (*header.ExtendedHeader)",
		"() -> ()",
		"() -> (<-chan *header.ExtendedHeader)",
	}
	methods := reflect.VisibleFields(reflect.TypeOf(TestFields{}))
	for i, method := range methods {
		require.Equal(t, testOutputs[i], parseSignatureForHelpstring(method))
	}
}

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

func parseSignatureForHelptring(methodSig reflect.StructField) string {
	simplifiedSignature := "("
	in, out := methodSig.Type.NumIn(), methodSig.Type.NumOut()
	for i := 1; i < in; i++ {
		simplifiedSignature += methodSig.Type.In(i).String()
		if i != in-1 {
			simplifiedSignature += ", "
		}
	}
	simplifiedSignature += ") -> ("
	for i := 0; i < out-1; i++ {
		simplifiedSignature += methodSig.Type.Out(i).String()
		if i != out-2 {
			simplifiedSignature += ", "
		}
	}
	simplifiedSignature += ")"
	return simplifiedSignature
}
