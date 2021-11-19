package header

import (
	"bytes"
	"context"
	"testing"

	libhost "github.com/libp2p/go-libp2p-core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	header_pb "github.com/celestiaorg/celestia-node/service/header/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

func TestP2PExchange_RequestHead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	host, peer := createMocknet(ctx, t)
	exchg, store := createExchangeWithMockStore(ctx, t, host, peer)
	// perform header request
	header, err := exchg.RequestHead(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.headers[len(store.headers)].Height, header.Height)
	assert.Equal(t, store.headers[len(store.headers)].Hash(), header.Hash())
}

func TestP2PExchange_RequestHeader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	host, peer := createMocknet(ctx, t)
	exchg, store := createExchangeWithMockStore(ctx, t, host, peer)
	// perform expected request
	header, err := exchg.RequestHeader(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, store.headers[5].Height, header.Height)
	assert.Equal(t, store.headers[5].Hash(), header.Hash())
}

func TestP2PExchange_RequestHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	host, peer := createMocknet(ctx, t)
	exchg, store := createExchangeWithMockStore(ctx, t, host, peer)
	// perform expected request
	gotHeaders, err := exchg.RequestHeaders(context.Background(), 1, 5)
	require.NoError(t, err)
	for _, got := range gotHeaders {
		assert.Equal(t, store.headers[int(got.Height)].Height, got.Height)
		assert.Equal(t, store.headers[int(got.Height)].Hash(), got.Hash())
	}
}

// TestP2PExchange_Response_Head tests that the P2PExchange instance can respond
// to an ExtendedHeaderRequest for the chain head.
func TestP2PExchange_Response_Head(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net, err := mocknet.FullMeshConnected(ctx, 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// create P2PExchange just to register the stream handler
	store := createStore(t, 5)
	ex := NewP2PExchange(host, libhost.InfoFromHost(peer), store)
	err = ex.Start(ctx)
	require.NoError(t, err)

	// start a new stream via Peer to see if Host can handle inbound requests
	stream, err := peer.NewStream(context.Background(), libhost.InfoFromHost(host).ID, exchangeProtocolID)
	require.NoError(t, err)
	// create request
	req := &header_pb.ExtendedHeaderRequest{
		Origin: uint64(0),
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
	eh, err := ProtoToExtendedHeader(resp)
	require.NoError(t, err)

	assert.Equal(t, store.headers[5].Height, eh.Height)
	assert.Equal(t, store.headers[5].Hash(), eh.Hash())
}

// TestP2PExchange_RequestByHash tests that the P2PExchange instance can
// respond to an ExtendedHeaderRequest for a hash instead of a height.
func TestP2PExchange_RequestByHash(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// create P2PExchange just to register the stream handler
	store := createStore(t, 5)
	ex := NewP2PExchange(host, libhost.InfoFromHost(peer), store)
	err = ex.Start(ctx)
	require.NoError(t, err)

	// start a new stream via Peer to see if Host can handle inbound requests
	stream, err := peer.NewStream(context.Background(), libhost.InfoFromHost(host).ID, exchangeProtocolID)
	require.NoError(t, err)
	// create request
	req := &header_pb.ExtendedHeaderRequest{
		Hash:   store.headers[3].Hash(),
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
	eh, err := ProtoToExtendedHeader(resp)
	require.NoError(t, err)

	assert.Equal(t, store.headers[3].Height, eh.Height)
	assert.Equal(t, store.headers[3].Hash(), eh.Hash())
}

// TestP2PExchange_Response_Single tests that the P2PExchange instance can respond
// to a ExtendedHeaderRequest for one ExtendedHeader accurately.
func TestExchange_Response_Single(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// create P2PExchange just to register the stream handler
	store := createStore(t, 5)
	ex := NewP2PExchange(host, libhost.InfoFromHost(peer), store)
	err = ex.Start(ctx)
	require.NoError(t, err)

	// start a new stream via Peer to see if Host can handle inbound requests
	stream, err := peer.NewStream(context.Background(), host.ID(), exchangeProtocolID)
	require.NoError(t, err)
	// create request
	origin := uint64(3)
	req := &ExtendedHeaderRequest{
		Origin: origin,
		Amount: 1,
	}
	// send request
	_, err = serde.Write(stream, req.ToProto())
	require.NoError(t, err)
	// read resp
	resp := new(header_pb.ExtendedHeader)
	_, err = serde.Read(stream, resp)
	require.NoError(t, err)
	// compare
	got, err := ProtoToExtendedHeader(resp)
	require.NoError(t, err)
	assert.Equal(t, store.headers[int(origin)].Height, got.Height)
	assert.Equal(t, store.headers[int(origin)].Hash(), got.Hash())
}

// TestP2PExchange_Response_Multiple tests that the P2PExchange instance can respond
// to a ExtendedHeaderRequest for multiple ExtendedHeaders accurately.
func TestExchange_Response_Multiple(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net, err := mocknet.FullMeshConnected(ctx, 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// create P2PExchange just to register the stream handler
	store := createStore(t, 5)
	ex := NewP2PExchange(host, libhost.InfoFromHost(peer), store)
	err = ex.Start(ctx)
	require.NoError(t, err)

	// start a new stream via Peer to see if Host can handle inbound requests
	stream, err := peer.NewStream(context.Background(), libhost.InfoFromHost(host).ID, exchangeProtocolID)
	require.NoError(t, err)
	// create request
	origin := uint64(3)
	req := &header_pb.ExtendedHeaderRequest{
		Origin: origin,
		Amount: 2,
	}
	// send request
	_, err = serde.Write(stream, req)
	require.NoError(t, err)
	// read responses
	for i := origin; i < (origin + req.Amount); i++ {
		resp := new(header_pb.ExtendedHeader)
		_, err := serde.Read(stream, resp)
		require.NoError(t, err)
		eh, err := ProtoToExtendedHeader(resp)
		require.NoError(t, err)
		// compare
		assert.Equal(t, store.headers[int(i)].Height, eh.Height)
		assert.Equal(t, store.headers[int(i)].Hash(), eh.Hash())
	}
}

func createMocknet(ctx context.Context, t *testing.T) (libhost.Host, libhost.Host) {
	net, err := mocknet.FullMeshConnected(ctx, 2)
	require.NoError(t, err)
	// get host and peer
	return net.Hosts()[0], net.Hosts()[1]
}

func createExchangeWithMockStore(ctx context.Context, t *testing.T, host, peer libhost.Host) (Exchange, *mockStore) {
	store := createStore(t, 5)
	// create P2PExchange on peer side to handle requests
	ex := NewP2PExchange(peer, libhost.InfoFromHost(host), store)
	err := ex.Start(ctx)
	require.NoError(t, err)

	// create new P2PExchange
	exchg := NewP2PExchange(host, libhost.InfoFromHost(peer), nil) // we don't need the store on the requesting side
	err = ex.Start(ctx)
	require.NoError(t, err)
	return exchg, store
}

type mockStore struct {
	headers map[int]*ExtendedHeader
	head    *ExtendedHeader
}

// createStore creates a mock store and adds several random
// headers
func createStore(t *testing.T, numHeaders int) *mockStore {
	store := &mockStore{
		headers: make(map[int]*ExtendedHeader, 4),
	}
	for i := 1; i <= numHeaders; i++ {
		store.headers[i] = RandExtendedHeader(t)
		store.headers[i].Height = int64(i)
	}
	store.head = store.headers[numHeaders]
	return store
}

func (m *mockStore) Head(context.Context) (*ExtendedHeader, error) {
	if m.head == nil {
		return nil, ErrNoHead
	}
	return m.head, nil
}

func (m *mockStore) Get(ctx context.Context, hash tmbytes.HexBytes) (*ExtendedHeader, error) {
	for _, header := range m.headers {
		if bytes.Equal(header.Hash(), hash) {
			return header, nil
		}
	}
	return nil, nil
}

func (m *mockStore) GetByHeight(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	return m.headers[int(height)], nil
}

func (m *mockStore) GetRangeByHeight(ctx context.Context, from, to uint64) ([]*ExtendedHeader, error) {
	headers := make([]*ExtendedHeader, to-from)
	for i := range headers {
		headers[i] = m.headers[int(from)]
		from++
	}
	return headers, nil
}

func (m *mockStore) Has(context.Context, tmbytes.HexBytes) (bool, error) {
	return false, nil
}

func (m *mockStore) Append(ctx context.Context, headers ...*ExtendedHeader) error {
	for i, header := range headers {
		m.headers[int(header.Height)] = header
		// set head
		if i == len(headers)-1 {
			m.head = header
		}
	}
	return nil
}
