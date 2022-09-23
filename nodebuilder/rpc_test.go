package nodebuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"go.uber.org/fx"

	das "github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/local"
	"github.com/celestiaorg/celestia-node/header/store"
	"github.com/celestiaorg/celestia-node/header/sync"
	headerServ "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	shareServ "github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/params"
	"github.com/celestiaorg/celestia-node/service/rpc"
	"github.com/celestiaorg/celestia-node/share"
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
	// ensure daser has run (or is still running)
	assert.True(t, dasStateResp.SampledChainHead == 10 || dasStateResp.IsRunning)
}

func setupNodeWithModifiedRPC(t *testing.T) *Node {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	// create test node with a dummy header headerServ, manually add a dummy header
	daser := setupDASer(t)
	hServ := setupHeaderService(ctx, t)
	// create overrides
	overrideHeaderServ := fx.Replace(hServ)
	// fx.Decorate(func() *das.DASer {return daser}) == fx.Replace(daser)
	overrideDASer := fx.Decorate(fx.Annotate(
		func() *das.DASer {
			return daser
		},
		fx.OnStart(func(ctx context.Context) error {
			return daser.Start(ctx)
		}),
		fx.OnStop(func(ctx context.Context) error {
			return daser.Stop(ctx)
		}),
	))
	overrideRPCHandler := fx.Invoke(func(srv *rpc.Server, state state.Service, share shareServ.Service) {
		handler := rpc.NewHandler(state, share, hServ, daser)
		handler.RegisterEndpoints(srv)
	})
	nd := TestNode(t, node.Full, overrideHeaderServ, overrideDASer, overrideRPCHandler)
	// start node
	err := nd.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = nd.Stop(ctx)
		require.NoError(t, err)
	})
	return nd
}

func setupHeaderService(ctx context.Context, t *testing.T) headerServ.Service {
	suite := header.NewTestSuite(t, 1)
	head := suite.Head()
	// create header stores
	remoteStore := store.NewTestStore(ctx, t, head)
	localStore := store.NewTestStore(ctx, t, head)
	_, err := localStore.Append(ctx, suite.GenExtendedHeaders(5)...)
	require.NoError(t, err)
	// create syncer
	syncer := sync.NewSyncer(local.NewExchange(remoteStore), localStore, &header.DummySubscriber{}, params.BlockTime)

	return headerServ.NewHeaderService(syncer, nil, nil, nil, localStore)
}

func setupDASer(t *testing.T) *das.DASer {
	suite := header.NewTestSuite(t, 1)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	sub := &header.DummySubscriber{Headers: suite.GenExtendedHeaders(10)}

	bServ := mdutils.Bserv()
	mockGet := &mockGetter{
		headers:        make(map[int64]*header.ExtendedHeader),
		doneCh:         make(chan struct{}),
		brokenHeightCh: make(chan struct{}),
	}

	mockGet.generateHeaders(t, bServ, 0, 10)
	return das.NewDASer(share.NewTestSuccessfulAvailability(), sub, mockGet, ds, nil)
}

// taken from daser_test.go
type mockGetter struct {
	getterStub
	doneCh chan struct{} // signals all stored headers have been retrieved

	brokenHeightCh chan struct{}

	head    int64
	headers map[int64]*header.ExtendedHeader
}

func (m *mockGetter) generateHeaders(t *testing.T, bServ blockservice.BlockService, startHeight, endHeight int) {
	for i := startHeight; i < endHeight; i++ {
		dah := share.RandFillBS(t, 16, bServ)

		randHeader := header.RandExtendedHeader(t)
		randHeader.DataHash = dah.Hash()
		randHeader.DAH = dah
		randHeader.Height = int64(i + 1)

		m.headers[int64(i+1)] = randHeader
	}
	// set network head
	m.head = int64(startHeight + endHeight)
}

func (m *mockGetter) Head(context.Context) (*header.ExtendedHeader, error) {
	return m.headers[m.head], nil
}

type getterStub struct{}

func (m getterStub) Head(context.Context) (*header.ExtendedHeader, error) {
	return nil, nil
}

func (m getterStub) GetByHeight(_ context.Context, height uint64) (*header.ExtendedHeader, error) {
	return &header.ExtendedHeader{
		RawHeader: header.RawHeader{Height: int64(height)},
		DAH:       &header.DataAvailabilityHeader{RowsRoots: make([][]byte, 0)}}, nil
}

func (m getterStub) GetRangeByHeight(ctx context.Context, from, to uint64) ([]*header.ExtendedHeader, error) {
	return nil, nil
}

func (m getterStub) Get(context.Context, tmbytes.HexBytes) (*header.ExtendedHeader, error) {
	return nil, nil
}
