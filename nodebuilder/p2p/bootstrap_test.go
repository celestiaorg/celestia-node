package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBootstrappersFor_UsesCliBootnodes tests that CLI bootnodes override hardcoded list
func TestBootstrappersFor_UsesCliBootnodes(t *testing.T) {
	bootnode1 := "/dnsaddr/bootstrap1.example.com/p2p/12D3KooWSqZaLcn5Guypo2mrHr297YPJnV8KMEMXNjs3qAS8msw8"
	bootnode2 := "/dnsaddr/bootstrap2.example.com/p2p/12D3KooWQpuTFELgsUypqp9N4a1rKBccmrmQVY8Em9yhqppTJcXf"

	// Pass CLI bootnodes explicitly
	booters, err := BootstrappersFor(Private, bootnode1, bootnode2)
	require.NoError(t, err)
	require.Len(t, booters, 2)

	// Verify the bootnodes were parsed correctly
	assert.Equal(t, bootnode1, booters[0].Addrs[0].String()+"/p2p/"+booters[0].ID.String())
	assert.Equal(t, bootnode2, booters[1].Addrs[0].String()+"/p2p/"+booters[1].ID.String())
}

// TestBootstrappersFor_UsesHardcodedWhenNoCli tests that hardcoded list is used when no CLI bootnodes provided
func TestBootstrappersFor_UsesHardcodedWhenNoCli(t *testing.T) {
	// For Mainnet, should have hardcoded bootnodes
	booters, err := BootstrappersFor(Mainnet)
	require.NoError(t, err)
	assert.True(t, len(booters) > 0, "Mainnet should have hardcoded bootnodes")
}

// TestBootstrappersFor_EmptyListForPrivateNetwork tests that Private network has empty hardcoded list
func TestBootstrappersFor_EmptyListForPrivateNetwork(t *testing.T) {
	// Private network should have no hardcoded bootnodes
	booters, err := BootstrappersFor(Private)
	require.NoError(t, err)
	assert.Len(t, booters, 0)
}

// TestBootstrappersFor_CliOverridesHardcoded tests that CLI bootnodes override hardcoded ones
func TestBootstrappersFor_CliOverridesHardcoded(t *testing.T) {
	bootnode := "/dnsaddr/custom.example.com/p2p/12D3KooWSqZaLcn5Guypo2mrHr297YPJnV8KMEMXNjs3qAS8msw8"

	// Pass CLI bootnode for Mainnet (which has hardcoded bootnodes)
	booters, err := BootstrappersFor(Mainnet, bootnode)
	require.NoError(t, err)

	// Should have only 1 bootnode (the custom one), not the hardcoded ones
	assert.Len(t, booters, 1)
}
