package p2p

import (
	"bytes"
	"context"
	"testing"

	libhost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/header"
	p2p_pb "github.com/celestiaorg/celestia-node/header/p2p/pb"
	header_pb "github.com/celestiaorg/celestia-node/header/pb"
)

func TestExchange_RequestHead(t *testing.T) {
	host, peer := createMocknet(t)
	exchg, store := createP2PExAndServer(t, host, peer)
	// perform header request
	header, err := exchg.Head(context.Background())
	require.NoError(t, err)

	assert.Equal(t, store.headers[store.headHeight].Height, header.Height)
	assert.Equal(t, store.headers[store.headHeight].Hash(), header.Hash())
}

func TestExchange_RequestHeader(t *testing.T) {
	host, peer := createMocknet(t)
	exchg, store := createP2PExAndServer(t, host, peer)
	// perform expected request
	header, err := exchg.GetByHeight(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, store.headers[5].Height, header.Height)
	assert.Equal(t, store.headers[5].Hash(), header.Hash())
}

func TestExchange_RequestHeaders(t *testing.T) {
	host, peer := createMocknet(t)
	exchg, store := createP2PExAndServer(t, host, peer)
	// perform expected request
	gotHeaders, err := exchg.GetRangeByHeight(context.Background(), 1, 5)
	require.NoError(t, err)
	for _, got := range gotHeaders {
		assert.Equal(t, store.headers[got.Height].Height, got.Height)
		assert.Equal(t, store.headers[got.Height].Hash(), got.Hash())
	}
}

// TestExchange_RequestByHash tests that the Exchange instance can
// respond to an ExtendedHeaderRequest for a hash instead of a height.
func TestExchange_RequestByHash(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// create and start the ExchangeServer
	store := createStore(t, 5)
	serv := NewExchangeServer(host, store)
	err = serv.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		serv.Stop(context.Background()) //nolint:errcheck
	})

	// start a new stream via Peer to see if Host can handle inbound requests
	stream, err := peer.NewStream(context.Background(), libhost.InfoFromHost(host).ID, exchangeProtocolID)
	require.NoError(t, err)
	// create request for a header at a random height
	reqHeight := store.headHeight - 2
	req := &p2p_pb.ExtendedHeaderRequest{
		Data:   &p2p_pb.ExtendedHeaderRequest_Hash{Hash: store.headers[reqHeight].Hash()},
		Amount: 1,
	}
	// send request
	_, err = serde.Write(stream, req)
	require.NoError(t, err)
	// read resp
	resp := new(header_pb.ExtendedHeader)
	_, err = serde.Read(stream, resp)
	require.NoError(t, err)
	// compare
	eh, err := header.ProtoToExtendedHeader(resp)
	require.NoError(t, err)

	assert.Equal(t, store.headers[reqHeight].Height, eh.Height)
	assert.Equal(t, store.headers[reqHeight].Hash(), eh.Hash())
}

func createMocknet(t *testing.T) (libhost.Host, libhost.Host) {
	net, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err)
	// get host and peer
	return net.Hosts()[0], net.Hosts()[1]
}

// createP2PExAndServer creates a Exchange with 5 headers already in its store.
func createP2PExAndServer(t *testing.T, host, tpeer libhost.Host) (header.Exchange, *mockStore) {
	store := createStore(t, 5)
	serverSideEx := NewExchangeServer(tpeer, store)
	err := serverSideEx.Start(context.Background())
	require.NoError(t, err)

	t.Cleanup(func() {
		serverSideEx.Stop(context.Background()) //nolint:errcheck
	})

	return NewExchange(host, []peer.ID{tpeer.ID()}), store
}

type mockStore struct {
	headers    map[int64]*header.ExtendedHeader
	headHeight int64
}

// createStore creates a mock store and adds several random
// headers
func createStore(t *testing.T, numHeaders int) *mockStore {
	store := &mockStore{
		headers:    make(map[int64]*header.ExtendedHeader),
		headHeight: 0,
	}

	suite := header.NewTestSuite(t, numHeaders)

	for i := 0; i < numHeaders; i++ {
		header := suite.GenExtendedHeader()
		store.headers[header.Height] = header

		if header.Height > store.headHeight {
			store.headHeight = header.Height
		}
	}
	return store
}

func (m *mockStore) Init(context.Context, *header.ExtendedHeader) error { return nil }
func (m *mockStore) Start(context.Context) error                        { return nil }
func (m *mockStore) Stop(context.Context) error                         { return nil }

func (m *mockStore) Height() uint64 {
	return uint64(m.headHeight)
}

func (m *mockStore) Head(context.Context) (*header.ExtendedHeader, error) {
	return m.headers[m.headHeight], nil
}

func (m *mockStore) Get(ctx context.Context, hash tmbytes.HexBytes) (*header.ExtendedHeader, error) {
	for _, header := range m.headers {
		if bytes.Equal(header.Hash(), hash) {
			return header, nil
		}
	}
	return nil, nil
}

func (m *mockStore) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	return m.headers[int64(height)], nil
}

func (m *mockStore) GetRangeByHeight(ctx context.Context, from, to uint64) ([]*header.ExtendedHeader, error) {
	headers := make([]*header.ExtendedHeader, to-from)
	for i := range headers {
		headers[i] = m.headers[int64(from)]
		from++
	}
	return headers, nil
}

func (m *mockStore) Has(context.Context, tmbytes.HexBytes) (bool, error) {
	return false, nil
}

func (m *mockStore) Append(ctx context.Context, headers ...*header.ExtendedHeader) (int, error) {
	for _, header := range headers {
		m.headers[header.Height] = header
		// set head
		if header.Height > m.headHeight {
			m.headHeight = header.Height
		}
	}
	return len(headers), nil
}
