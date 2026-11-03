package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/state"
)

// TestDelegateCommandsArgCount verifies that delegate and undelegate accept the
// validator address and amount they document, and nothing else.
func TestDelegateCommandsArgCount(t *testing.T) {
	cmds := []*cobra.Command{delegateCmd, undelegateCmd}
	for _, cmd := range cmds {
		t.Run(cmd.Name(), func(t *testing.T) {
			require.NoError(t, cmd.ValidateArgs([]string{"valAddress", "100"}))
			require.Error(t, cmd.ValidateArgs([]string{"valAddress"}))
			require.Error(t, cmd.ValidateArgs([]string{"valAddress", "100", "extra"}))
		})
	}
}

// TestParseAddressKind verifies that commands get an error, not a panic, when given the wrong
// kind of address.
func TestParseAddressKind(t *testing.T) {
	accStr := state.AccAddress(make([]byte, 20)).String()
	valStr := state.ValAddress(make([]byte, 20)).String()

	acc, err := parseAccAddress(accStr)
	require.NoError(t, err)
	require.Equal(t, accStr, acc.String())
	_, err = parseAccAddress(valStr)
	require.ErrorContains(t, err, "expected an account address")

	val, err := parseValAddress(valStr)
	require.NoError(t, err)
	require.Equal(t, valStr, val.String())
	_, err = parseValAddress(accStr)
	require.ErrorContains(t, err, "expected a validator address")
}
