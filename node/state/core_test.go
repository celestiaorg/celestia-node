package state

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
	core_components "github.com/celestiaorg/celestia-node/node/core"
)

func TestChainClient(t *testing.T) {
	nd, _, ip := core.StartRemoteCore()
	//nolint:errcheck
	defer nd.Stop()

	cfg := core_components.DefaultConfig()
	cfg.RemoteConfig.RemoteAddr = ip
	cfg.RemoteConfig.Protocol = "http"
	cfg.Remote = true

	dir, err := ioutil.TempDir("", "prefix")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	cc, err := ChainClient(cfg, dir)
	require.NoError(t, err)
	require.NotNil(t, cc)

	latestHeight, err := cc.QueryLatestHeight()
	require.NoError(t, err)
	assert.Equal(t, int64(0), latestHeight)
}
