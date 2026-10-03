package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestDelegationCommandsAcceptDocumentedOperands(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "delegate", cmd: delegateCmd},
		{name: "undelegate", cmd: undelegateCmd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.cmd.Args(tt.cmd, []string{"validator_address", "1000"}))
			require.Error(t, tt.cmd.Args(tt.cmd, []string{"validator_address"}))
			require.Error(t, tt.cmd.Args(tt.cmd, []string{"validator_address", "1000", "extra"}))
		})
	}
}
