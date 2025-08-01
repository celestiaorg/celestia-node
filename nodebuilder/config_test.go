package nodebuilder

import (
	"bytes"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// TestConfigWriteRead tests that the configs for all node types can be encoded to and from TOML.
func TestConfigWriteRead(t *testing.T) {
	tests := []node.Type{
		node.Full,
		node.Light,
		node.Bridge,
	}

	for _, tp := range tests {
		t.Run(tp.String(), func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			in := DefaultConfig(tp)

			err := in.Encode(buf)
			require.NoError(t, err)

			var out Config
			err = out.Decode(buf)
			require.NoError(t, err)
			assert.EqualValues(t, in, &out)
		})
	}
}

// TestUpdateConfig tests that updating an outdated config
// using a new default config applies the correct values and
// preserves old custom values.
func TestUpdateConfig(t *testing.T) {
	cfg := new(Config)
	_, err := toml.Decode(outdatedConfig, cfg)
	require.NoError(t, err)

	newCfg := DefaultConfig(node.Light)
	// ensure this config field is not filled in the outdated config
	require.NotEqual(t, newCfg.Share.PeerManagerParams, cfg.Share.PeerManagerParams)

	cfg, err = updateConfig(cfg, newCfg)
	require.NoError(t, err)
	// ensure this config field is now set after updating the config
	require.Equal(t, newCfg.Share.PeerManagerParams, cfg.Share.PeerManagerParams)
	// ensure old custom values were not changed
	require.Equal(t, "thisshouldnthavechanged", cfg.State.DefaultKeyName)
	require.Equal(t, "7979", cfg.RPC.Port)
}

// outdatedConfig is an outdated config from a light node
var outdatedConfig = `
[Core]
  IP = "0.0.0.0"
  Port = "0"

[State]
  DefaultKeyName = "thisshouldnthavechanged"
  DefaultBackendName = "test"

[P2P]
  ListenAddresses = ["/ip4/0.0.0.0/udp/2121/quic-v1", "/ip6/::/udp/2121/quic-v1", "/ip4/0.0.0.0/tcp/2121",
"/ip6/::/tcp/2121"]
  AnnounceAddresses = []
  NoAnnounceAddresses = ["/ip4/0.0.0.0/udp/2121/quic-v1", "/ip4/127.0.0.1/udp/2121/quic-v1", "/ip6/::/udp/2121/quic-v1",
"/ip4/0.0.0.0/tcp/2121", "/ip4/127.0.0.1/tcp/2121", "/ip6/::/tcp/2121"]
  MutualPeers = []
  PeerExchange = false
  RoutingTableRefreshPeriod = "1m0s"
  [P2P.ConnManager]
    Low = 50
    High = 100
    GracePeriod = "1m0s"
  [P2P.Metrics]
    PrometheusAgentPort = "8890"

[RPC]
  Address = "0.0.0.0"
  Port = "7979"

[Share]
  PeersLimit = 5
  DiscoveryInterval = "30s"
  AdvertiseInterval = "30s"
  UseShareExchange = true
 [Share.ShrexClient]
    ReadTimeout = "2m0s"
    WriteTimeout = "5s"
  [Share.ShrexServer]   
    ReadTimeout = "5s"
    WriteTimeout = "1m0s"
    HandleRequestTimeout = "1m0s"
    ConcurrencyLimit = 10

[Header]
  TrustedHash = ""
  TrustedPeers = []
  [Header.Store]
    StoreCacheSize = 4096
    IndexCacheSize = 16384
    WriteBatchSize = 2048
  [Header.Syncer]
    TrustingPeriod = "168h0m0s"
  [Header.Server]
    WriteDeadline = "8s"
    ReadDeadline = "1m0s"
    RangeRequestTimeout = "10s"
  [Header.Client]
    MaxHeadersPerRangeRequest = 64
    RangeRequestTimeout = "8s"
    TrustedPeersRequestTimeout = "300ms"

[DASer]
  SamplingRange = 100
  ConcurrencyLimit = 16
  BackgroundStoreInterval = "10m0s"
  SampleFrom = 1
  SampleTimeout = "4m0s"
`
