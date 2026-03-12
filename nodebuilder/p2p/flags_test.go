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

// Set empty network flag and ensure error returned
func TestParseNetwork_emptyFlag(t *testing.T) {
	cmd := createCmdWithNetworkFlag()

	err := cmd.Flags().Set(networkFlag, "")
	require.NoError(t, err)

	_, err = ParseNetwork(cmd)
	assert.Error(t, err)
}

// Set empty network flag and ensure error returned
func TestParseNetwork_emptyEnvEmptyFlag(t *testing.T) {
	t.Setenv(EnvCustomNetwork, "")

	cmd := createCmdWithNetworkFlag()
	err := cmd.Flags().Set(networkFlag, "")
	require.NoError(t, err)

	_, err = ParseNetwork(cmd)
	require.Error(t, err)
}

// Env overrides empty flag to take precedence
func TestParseNetwork_envOverridesEmptyFlag(t *testing.T) {
	t.Setenv(EnvCustomNetwork, "custom-network")

	cmd := createCmdWithNetworkFlag()
	err := cmd.Flags().Set(networkFlag, "")
	require.NoError(t, err)

	network, err := ParseNetwork(cmd)
	require.NoError(t, err)
	assert.Equal(t, Network("custom-network"), network)
}

// Explicitly set flag but env should still override
func TestParseNetwork_envOverridesFlag(t *testing.T) {
	t.Setenv(EnvCustomNetwork, "custom-network")

	cmd := createCmdWithNetworkFlag()
	err := cmd.Flags().Set(networkFlag, string(Mocha))
	require.NoError(t, err)

	network, err := ParseNetwork(cmd)
	require.NoError(t, err)
	assert.Equal(t, Network("custom-network"), network)
}

// TestParseFlags_BootnodesParsed tests that --p2p.bootnodes flag is correctly parsed
func TestParseFlags_BootnodesParsed(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().AddFlagSet(Flags())

	bootnode1 := "/dnsaddr/bootstrap1.example.com/p2p/12D3KooWSqZaLcn5Guypo2mrHr297YPJnV8KMEMXNjs3qAS8msw8"
	bootnode2 := "/dnsaddr/bootstrap2.example.com/p2p/12D3KooWQpuTFELgsUypqp9N4a1rKBccmrmQVY8Em9yhqppTJcXf"

	err := cmd.Flags().Set(bootnodeFlag, bootnode1)
	require.NoError(t, err)
	err = cmd.Flags().Set(bootnodeFlag, bootnode2)
	require.NoError(t, err)

	cfg := DefaultConfig(1) // 1 = Bridge node type
	err = ParseFlags(cmd, &cfg)
	require.NoError(t, err)

	assert.Len(t, cfg.BootstrapPeers, 2)
	assert.Equal(t, bootnode1, cfg.BootstrapPeers[0])
	assert.Equal(t, bootnode2, cfg.BootstrapPeers[1])
}

// TestParseFlags_BootnodesInvalidMultiaddr tests that invalid multiaddr is rejected
func TestParseFlags_BootnodesInvalidMultiaddr(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().AddFlagSet(Flags())

	err := cmd.Flags().Set(bootnodeFlag, "not-a-valid-multiaddr")
	require.NoError(t, err)

	cfg := DefaultConfig(1)
	err = ParseFlags(cmd, &cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), bootnodeFlag)
}

// TestParseFlags_BootnodesEmpty tests that empty bootnodes list doesn't override config
func TestParseFlags_BootnodesEmpty(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().AddFlagSet(Flags())

	cfg := DefaultConfig(1)
	cfg.BootstrapPeers = []string{"existing-bootnode"}

	err := ParseFlags(cmd, &cfg)
	require.NoError(t, err)

	// Should keep existing bootstrap peers since no flag was set
	assert.Len(t, cfg.BootstrapPeers, 1)
	assert.Equal(t, "existing-bootnode", cfg.BootstrapPeers[0])
}
