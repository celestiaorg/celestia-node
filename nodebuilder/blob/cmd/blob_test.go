package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubmitPreRunE(t *testing.T) {
	args := []string{"namespace", "blobData"}
	require.NoError(t, submitCmd.PreRunE(submitCmd, args))
	require.Equal(t, []string{"0xnamespace", "blobData"}, args)

	// with --input-file the namespaces come from the file, so there are no arguments
	require.NoError(t, submitCmd.PreRunE(submitCmd, nil))
}
