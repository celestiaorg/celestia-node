package header

import (
	"context"
	"testing"

	libhost "github.com/libp2p/go-libp2p-core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmbytes "github.com/celestiaorg/celestia-core/libs/bytes"
	header_pb "github.com/celestiaorg/celestia-node/service/header/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

func TestExchange_RequestHead(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// create exchange on peer side to handle requests
	store := createStore(t, 5)
	_ = newExchange(peer, libhost.InfoFromHost(host), store)
	// create new exchange
	exchg := newExchange(host, libhost.InfoFromHost(peer), nil) // we don't need the store on the requesting side
	// perform header request
	header, err := exchg.RequestHead(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.headers[len(store.headers)].Height, header.Height)
	assert.Equal(t, store.headers[len(store.headers)].Hash(), header.Hash())
}

func TestExchange_RequestHeader(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// create exchange on peer side to handle requests
	store := createStore(t, 5)
	_ = newExchange(peer, libhost.InfoFromHost(host), store)
	// create new exchange
	exchg := newExchange(host, libhost.InfoFromHost(peer), new(mockStore))
	// perform expected request
	header, err := exchg.RequestHeader(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, store.headers[5].Height, header.Height)
	assert.Equal(t, store.headers[5].Hash(), header.Hash())
}

func TestExchange_RequestHeaders(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// create exchange just to register the stream handler
	store := createStore(t, 5)
	_ = newExchange(peer, libhost.InfoFromHost(host), store)
	// create new exchange
	exchg := newExchange(host, libhost.InfoFromHost(peer), nil)
	// perform expected request
	gotHeaders, err := exchg.RequestHeaders(context.Background(), 1, 5)
	require.NoError(t, err)
	for _, got := range gotHeaders {
		assert.Equal(t, store.headers[int(got.Height)].Height, got.Height)
		assert.Equal(t, store.headers[int(got.Height)].Hash(), got.Hash())
	}
}

// TestExchange_Response_Head tests that the exchange instance can respond
// to an ExtendedHeaderRequest for the chain head.
func TestExchange_Response_Head(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// create exchange just to register the stream handler
	store := createStore(t, 5)
	_ = newExchange(host, libhost.InfoFromHost(peer), store)
	// start a new stream via Peer to see if Host can handle inbound requests
	stream, err := peer.NewStream(context.Background(), libhost.InfoFromHost(host).ID, headerExchangeProtocolID)
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

// TestExchange_Response_Single tests that the exchange instance can respond
// to a ExtendedHeaderRequest for one ExtendedHeader accurately.
func TestExchange_Response_Single(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// create exchange just to register the stream handler
	store := createStore(t, 5)
	_ = newExchange(host, libhost.InfoFromHost(peer), store)
	// start a new stream via Peer to see if Host can handle inbound requests
	stream, err := peer.NewStream(context.Background(), libhost.InfoFromHost(host).ID, headerExchangeProtocolID)
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

// TestExchange_Response_Multiple tests that the exchange instance can respond
// to a ExtendedHeaderRequest for multiple ExtendedHeaders accurately.
func TestExchange_Response_Multiple(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// create exchange just to register the stream handler
	store := createStore(t, 5)
	_ = newExchange(host, libhost.InfoFromHost(peer), store)
	// start a new stream via Peer to see if Host can handle inbound requests
	stream, err := peer.NewStream(context.Background(), libhost.InfoFromHost(host).ID, headerExchangeProtocolID)
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

type mockStore struct {
	headers map[int]*ExtendedHeader
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
	return store
}

func (m *mockStore) Head() (*ExtendedHeader, error) {
	return m.headers[len(m.headers)], nil
}

func (m *mockStore) Get(ctx context.Context, hash tmbytes.HexBytes) (*ExtendedHeader, error) {
	return nil, nil
}

func (m *mockStore) GetMany(ctx context.Context, hashes []tmbytes.HexBytes) ([]*ExtendedHeader, error) {
	return nil, nil
}

func (m *mockStore) GetByHeight(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	return m.headers[int(height)], nil
}

func (m *mockStore) GetRangeByHeight(ctx context.Context, from, to uint64) ([]*ExtendedHeader, error) {
	return nil, nil
}

func (m *mockStore) Put(ctx context.Context, header *ExtendedHeader) error {
	return nil
}
func (m *mockStore) PutMany(ctx context.Context, headers []*ExtendedHeader) error {
	return nil
}
