package header

import (
	"context"
	"testing"

	libhost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmbytes "github.com/celestiaorg/celestia-core/libs/bytes"
)

func TestExchange_RequestHead(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// set expected value
	expected = RandExtendedHeader(t)
	expected.Height = 8
	// create stream + stream handler
	peer.SetStreamHandler(headerExchangeProtocolID, testHeaderHandler)
	// create new exchange
	exchg := newExchange(host, libhost.InfoFromHost(peer), new(mockStore))
	// perform expected request
	header, err := exchg.RequestHead(context.Background())
	require.NoError(t, err)
	assert.Equal(t, expected.Height, header.Height)
	assert.Equal(t, expected.Hash(), header.Hash())
}

func TestExchange_RequestHeader(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// set expected value
	expected = RandExtendedHeader(t)
	expected.Height = 5
	// create stream + stream handler
	peer.SetStreamHandler(headerExchangeProtocolID, testHeaderHandler)
	// create new exchange
	exchg := newExchange(host, libhost.InfoFromHost(peer), new(mockStore))
	// perform expected request
	header, err := exchg.RequestHeader(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, expected.Height, header.Height)
	assert.Equal(t, expected.Hash(), header.Hash())
}

func TestExchange_RequestHeaders(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// set multipleExpected value
	for i := 0; i <= 4; i++ {
		multipleExpected[i] = RandExtendedHeader(t)
		multipleExpected[i].Height = int64(i + 1)
	}
	// create stream + stream handler
	peer.SetStreamHandler(headerExchangeProtocolID, testMultipleHeadersHandler)
	// create new exchange
	exchg := newExchange(host, libhost.InfoFromHost(peer), new(mockStore))
	// perform expected request
	gotHeaders, err := exchg.RequestHeaders(context.Background(), 5, 5)
	require.NoError(t, err)
	for i, got := range gotHeaders {
		assert.Equal(t, multipleExpected[i].Height, got.Height)
		assert.Equal(t, multipleExpected[i].Hash(), got.Hash())
	}
}

var expected *ExtendedHeader
var multipleExpected = make([]*ExtendedHeader, 5)

func testHeaderHandler(stream network.Stream) {
	bin, err := expected.MarshalBinary()
	if err != nil {
		panic(err)
	}
	_, err = stream.Write(bin)
	if err != nil {
		panic(err)
	}
}

func testMultipleHeadersHandler(stream network.Stream) {
	for _, header := range multipleExpected {
		bin, err := header.MarshalBinary()
		if err != nil {
			panic(err)
		}
		_, err = stream.Write(bin)
		if err != nil {
			panic(err)
		}
	}
}

// TestExchange_Response_Head tests that the exchange instance can respond
// to an ExtendedHeaderRequest for the chain head.
func TestExchange_Response_Head(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// fill out mockstore
	store := new(mockStore)
	store.headers = make(map[int]*ExtendedHeader)
	for i := 1; i <= 5; i++ {
		store.headers[i] = RandExtendedHeader(t)
		store.headers[i].Height = int64(i)
	}
	// create exchange just to register the stream handler
	_ = newExchange(host, libhost.InfoFromHost(peer), store)
	// start a new stream via Peer to see if Host can handle inbound requests
	stream, err := peer.NewStream(context.Background(), libhost.InfoFromHost(host).ID, headerExchangeProtocolID)
	require.NoError(t, err)
	// create request
	req := &ExtendedHeaderRequest{
		Origin: uint64(0),
		Amount: 1,
	}
	bin, err := req.MarshalBinary()
	require.NoError(t, err)
	// send request
	_, err = stream.Write(bin)
	require.NoError(t, err)
	// read resp
	buf := make([]byte, 2000)
	respSize, err := stream.Read(buf)
	require.NoError(t, err)
	resp := new(ExtendedHeader)
	err = resp.UnmarshalBinary(buf[:respSize])
	require.NoError(t, err)
	// compare
	assert.Equal(t, store.headers[5].Height, resp.Height)
	assert.Equal(t, store.headers[5].Hash(), resp.Hash())
}

// TestExchange_Response_Single tests that the exchange instance can respond
// to a ExtendedHeaderRequest for one ExtendedHeader accurately.
func TestExchange_Response_Single(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// fill out mockstore
	store := new(mockStore)
	store.headers = make(map[int]*ExtendedHeader)
	for i := 1; i <= 5; i++ {
		store.headers[i] = RandExtendedHeader(t)
		store.headers[i].Height = int64(i)
	}
	// create exchange just to register the stream handler
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
	bin, err := req.MarshalBinary()
	require.NoError(t, err)
	// send request
	_, err = stream.Write(bin)
	require.NoError(t, err)
	// read resp
	buf := make([]byte, 2000)
	respSize, err := stream.Read(buf)
	require.NoError(t, err)
	resp := new(ExtendedHeader)
	err = resp.UnmarshalBinary(buf[:respSize])
	require.NoError(t, err)
	// compare
	assert.Equal(t, store.headers[int(origin)].Height, resp.Height)
	assert.Equal(t, store.headers[int(origin)].Hash(), resp.Hash())
}

// TestExchange_Response_Multiple tests that the exchange instance can respond
// to a ExtendedHeaderRequest for multiple ExtendedHeaders accurately.
func TestExchange_Response_Multiple(t *testing.T) {
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)
	// get host and peer
	host, peer := net.Hosts()[0], net.Hosts()[1]
	// fill out mockstore
	store := new(mockStore)
	store.headers = make(map[int]*ExtendedHeader)
	for i := 1; i <= 5; i++ {
		store.headers[i] = RandExtendedHeader(t)
		store.headers[i].Height = int64(i)
	}
	// create exchange just to register the stream handler
	_ = newExchange(host, libhost.InfoFromHost(peer), store)
	// start a new stream via Peer to see if Host can handle inbound requests
	stream, err := peer.NewStream(context.Background(), libhost.InfoFromHost(host).ID, headerExchangeProtocolID)
	require.NoError(t, err)
	// create request
	origin := uint64(3)
	req := &ExtendedHeaderRequest{
		Origin: origin,
		Amount: 2,
	}
	bin, err := req.MarshalBinary()
	require.NoError(t, err)
	// send request
	_, err = stream.Write(bin)
	require.NoError(t, err)
	// read responses
	for i := origin; i < (origin + req.Amount); i++ {
		buf := make([]byte, 2000)
		respSize, err := stream.Read(buf)
		require.NoError(t, err)
		resp := new(ExtendedHeader)
		err = resp.UnmarshalBinary(buf[:respSize])
		require.NoError(t, err)
		// compare
		assert.Equal(t, store.headers[int(i)].Height, resp.Height)
		assert.Equal(t, store.headers[int(i)].Hash(), resp.Hash())
	}
}

type mockStore struct {
	headers map[int]*ExtendedHeader
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
