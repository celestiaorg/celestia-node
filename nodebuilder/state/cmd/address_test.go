package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/state"
)

func TestParseAddressKind(t *testing.T) {
	accAddr := state.AccAddress(bytes.Repeat([]byte{1}, 20)).String()
	valAddr := state.ValAddress(bytes.Repeat([]byte{2}, 20)).String()

	t.Run("account address", func(t *testing.T) {
		got, err := parseAccAddressFromString(accAddr)
		require.NoError(t, err)
		require.Equal(t, accAddr, got.String())

		_, err = parseValAddressFromString(accAddr)
		require.ErrorContains(t, err, "is not a validator address")
	})

	t.Run("validator address", func(t *testing.T) {
		got, err := parseValAddressFromString(valAddr)
		require.NoError(t, err)
		require.Equal(t, valAddr, got.String())

		_, err = parseAccAddressFromString(valAddr)
		require.ErrorContains(t, err, "is not an account address")
	})

	t.Run("invalid address", func(t *testing.T) {
		_, err := parseAccAddressFromString("nonsense")
		require.Error(t, err)

		_, err = parseValAddressFromString("nonsense")
		require.Error(t, err)
	})
}

// TestDelegateCommandsRejectAccountAddress checks that the commands report a bad
// address instead of panicking on the address kind assertion.
func TestDelegateCommandsRejectAccountAddress(t *testing.T) {
	accAddr := state.AccAddress(bytes.Repeat([]byte{1}, 20)).String()

	// the commands read the RPC client that InitClient installs in the context
	Cmd.PersistentFlags().AddFlagSet(cmdnode.RPCFlags())
	Cmd.SilenceErrors, Cmd.SilenceUsage = true, true
	t.Cleanup(func() {
		for _, name := range []string{"url", "token"} {
			f := Cmd.PersistentFlags().Lookup(name)
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})

	for _, cmd := range []*cobra.Command{delegateCmd, undelegateCmd} {
		t.Run(cmd.Name(), func(t *testing.T) {
			Cmd.SetArgs([]string{
				cmd.Name(), accAddr, "100",
				"--url", "http://127.0.0.1:1", "--token", "test",
			})
			err := Cmd.ExecuteContext(context.Background())
			require.ErrorContains(t, err, "is not a validator address")
		})
	}
}
