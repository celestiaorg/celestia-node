package p2p

import (
	"testing"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseNetwork_matchesByAlias checks to ensure flag parsing
// correctly matches the network's alias to the network name.
func TestParseNetwork_matchesByAlias(t *testing.T) {
	cmd := createCmdWithNetworkFlag()

	err := cmd.Flags().Set(networkFlag, "arabica")
	require.NoError(t, err)

	net, err := ParseNetwork(cmd)
	require.NoError(t, err)
	assert.Equal(t, Arabica, net)
}

// TestParseNetwork_matchesByValue checks to ensure flag parsing
// correctly matches the network's actual value to the network name.
func TestParseNetwork_matchesByValue(t *testing.T) {
	cmd := createCmdWithNetworkFlag()

	err := cmd.Flags().Set(networkFlag, string(Arabica))
	require.NoError(t, err)

	net, err := ParseNetwork(cmd)
	require.NoError(t, err)
	assert.Equal(t, Arabica, net)
}

// TestParseNetwork_parsesFromEnv checks to ensure flag parsing
// correctly fetches the value from the environment variable.
func TestParseNetwork_parsesFromEnv(t *testing.T) {
	cmd := createCmdWithNetworkFlag()

	t.Setenv(EnvCustomNetwork, "testing")

	net, err := ParseNetwork(cmd)
	require.NoError(t, err)
	assert.Equal(t, Network("testing"), net)
}

func TestParsedNetwork_invalidNetwork(t *testing.T) {
	cmd := createCmdWithNetworkFlag()

	err := cmd.Flags().Set(networkFlag, "invalid")
	require.NoError(t, err)

	net, err := ParseNetwork(cmd)
	assert.Error(t, err)
	assert.Equal(t, Network(""), net)
}

func createCmdWithNetworkFlag() *cobra.Command {
	cmd := &cobra.Command{}
	flags := &flag.FlagSet{}
	flags.String(
		networkFlag,
		"",
		"",
	)
	cmd.Flags().AddFlagSet(flags)
	return cmd
}
