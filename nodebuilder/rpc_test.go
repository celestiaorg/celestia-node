package nodebuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/local"
	"github.com/celestiaorg/celestia-node/header/store"
	"github.com/celestiaorg/celestia-node/header/sync"
	service "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/params"
	"github.com/celestiaorg/celestia-node/service/rpc"
)

// NOTE: The following tests are against common RPC endpoints provided by
// celestia-node. They will be removed upon refactoring of the RPC
// architecture and Public API. @renaynay @Wondertan.

const testHeight = uint64(2)

// TestNamespacedSharesRequest tests the `/namespaced_shares` endpoint.
func TestNamespacedSharesRequest(t *testing.T) {
	testGetNamespacedRequest(t, "namespaced_shares", func(t *testing.T, resp *http.Response) {
		t.Helper()
		namespacedShares := new(rpc.NamespacedSharesResponse)
		err := json.NewDecoder(resp.Body).Decode(namespacedShares)
		assert.NoError(t, err)
		assert.Equal(t, testHeight, namespacedShares.Height)
	})
}

// TestNamespacedDataRequest tests the `/namespaced_shares` endpoint.
func TestNamespacedDataRequest(t *testing.T) {
	testGetNamespacedRequest(t, "namespaced_data", func(t *testing.T, resp *http.Response) {
		t.Helper()
		namespacedData := new(rpc.NamespacedDataResponse)
		err := json.NewDecoder(resp.Body).Decode(namespacedData)
		assert.NoError(t, err)
		assert.Equal(t, testHeight, namespacedData.Height)
	})
}

func testGetNamespacedRequest(t *testing.T, endpointName string, assertResponseOK func(*testing.T, *http.Response)) {
	t.Helper()
	nd := setupNodeWithModifiedRPC(t)
	// create several requests for header at height 2
	var tests = []struct {
		nID         string
		expectedErr bool
		errMsg      string
	}{
		{
			nID:         "0000000000000001",
			expectedErr: false,
		},
		{
			nID:         "00000000000001",
			expectedErr: true,
			errMsg:      "expected namespace ID of size 8, got 7",
		},
		{
			nID:         "000000000000000001",
			expectedErr: true,
			errMsg:      "expected namespace ID of size 8, got 9",
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			endpoint := fmt.Sprintf("http://127.0.0.1:%s/%s/%s/height/%d",
				nd.RPCServer.ListenAddr()[5:], endpointName, tt.nID, testHeight)
			resp, err := http.Get(endpoint)
			defer func() {
				err = resp.Body.Close()
				require.NoError(t, err)
			}()
			// check resp
			if tt.expectedErr {
				require.False(t, resp.StatusCode == http.StatusOK)
				require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

				var errorMessage string
				err := json.NewDecoder(resp.Body).Decode(&errorMessage)

				require.NoError(t, err)
				require.Equal(t, tt.errMsg, errorMessage)

				return
			}
			require.NoError(t, err)
			require.True(t, resp.StatusCode == http.StatusOK)

			assertResponseOK(t, resp)
		})
	}
}

// TestHeadRequest rests the `/head` endpoint.
func TestHeadRequest(t *testing.T) {
	nd := setupNodeWithModifiedRPC(t)
	endpoint := fmt.Sprintf("http://127.0.0.1:%s/head", nd.RPCServer.ListenAddr()[5:])
	resp, err := http.Get(endpoint)
	require.NoError(t, err)
	defer func() {
		err = resp.Body.Close()
		require.NoError(t, err)
	}()
	require.True(t, resp.StatusCode == http.StatusOK)
}

// TestHeaderRequest tests the `/header` endpoint.
func TestHeaderRequest(t *testing.T) {
	nd := setupNodeWithModifiedRPC(t)
	// create several requests for headers
	var tests = []struct {
		height      uint64
		expectedErr bool
	}{
		{
			height:      uint64(2),
			expectedErr: false,
		},
		{
			height:      uint64(0),
			expectedErr: true,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			endpoint := fmt.Sprintf("http://127.0.0.1:%s/header/%d", nd.RPCServer.ListenAddr()[5:], tt.height)
			resp, err := http.Get(endpoint)
			require.NoError(t, err)
			defer func() {
				err = resp.Body.Close()
				require.NoError(t, err)
			}()

			require.Equal(t, tt.expectedErr, resp.StatusCode != http.StatusOK)
		})
	}
}

// TestAvailabilityRequest tests the /data_available endpoint.
func TestAvailabilityRequest(t *testing.T) {
	nd := setupNodeWithModifiedRPC(t)

	height := 5
	endpoint := fmt.Sprintf("http://127.0.0.1:%s/data_available/%d", nd.RPCServer.ListenAddr()[5:], height)
	resp, err := http.Get(endpoint)
	require.NoError(t, err)
	defer func() {
		err = resp.Body.Close()
		require.NoError(t, err)
	}()

	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	availResp := new(rpc.AvailabilityResponse)
	err = json.Unmarshal(buf, &availResp)
	require.NoError(t, err)

	assert.True(t, availResp.Available)
}

func TestDASStateRequest(t *testing.T) {
	nd := setupNodeWithModifiedRPC(t)

	endpoint := fmt.Sprintf("http://127.0.0.1:%s/daser/state", nd.RPCServer.ListenAddr()[5:])
	resp, err := http.Get(endpoint)
	require.NoError(t, err)
	defer func() {
		err = resp.Body.Close()
		require.NoError(t, err)
	}()
	dasStateResp := new(das.SamplingStats)
	err = json.NewDecoder(resp.Body).Decode(dasStateResp)
	require.NoError(t, err)
	// ensure daser is running
	assert.True(t, dasStateResp.IsRunning)
}

func setupNodeWithModifiedRPC(t *testing.T) *Node {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	// create test node with a dummy header service, manually add a dummy header
	hServ := setupHeaderService(ctx, t)
	// create overrides
	overrideHeaderServ := fx.Replace(hServ)
	nd := TestNode(t, node.Full, overrideHeaderServ)
	// start node
	err := nd.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = nd.Stop(ctx)
		require.NoError(t, err)
	})
	return nd
}

func setupHeaderService(ctx context.Context, t *testing.T) service.Service {
	suite := header.NewTestSuite(t, 1)
	head := suite.Head()
	// create header stores
	remoteStore := store.NewTestStore(ctx, t, head)
	localStore := store.NewTestStore(ctx, t, head)
	_, err := localStore.Append(ctx, suite.GenExtendedHeaders(5)...)
	require.NoError(t, err)
	// create syncer
	syncer := sync.NewSyncer(local.NewExchange(remoteStore), localStore, &header.DummySubscriber{}, params.BlockTime)

	return service.NewHeaderService(syncer, nil, nil, nil, localStore)
}
