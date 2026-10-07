package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
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
