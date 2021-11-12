package header

import (
	"context"
	"fmt"

	pb "github.com/celestiaorg/celestia-node/service/header/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
)

var headerExchangeProtocolID = protocol.ID("header-exchange")

// exchange enables sending outbound ExtendedHeaderRequests as well as
// handling inbound ExtendedHeaderRequests.
type exchange struct {
	host host.Host
	// TODO @renaynay: post-Devnet, we need to remove reliance of Exchange on one bootstrap peer
	// Ref https://github.com/celestiaorg/celestia-node/issues/172#issuecomment-964306823.
	peer  *peer.AddrInfo
	store Store
}

func newExchange(host host.Host, peer *peer.AddrInfo, store Store) *exchange {
	ex := &exchange{
		host:  host,
		peer:  peer,
		store: store,
	}
	host.SetStreamHandler(headerExchangeProtocolID, ex.requestHandler)
	return ex
}

// requestHandler handles inbound ExtendedHeaderRequests.
func (e *exchange) requestHandler(stream network.Stream) {
	// unmarshal request
	pbreq := new(pb.ExtendedHeaderRequest)
	_, err := serde.Read(stream, pbreq)
	if err != nil {
		log.Errorw("reading header request from stream", "err", err)
		//nolint:errcheck
		stream.Reset()
		return
	}
	// retrieve and write ExtendedHeaders
	height := pbreq.Origin
	amount := uint64(0)
	for amount < pbreq.Amount {
		e.handleRequest(height, stream)
		height++
		amount++
	}
	stream.Close()
}

// handleRequest fetches the ExtendedHeader at the given origin and
// writes it to the stream.
func (e *exchange) handleRequest(origin uint64, stream network.Stream) {
	var (
		header *ExtendedHeader
		err    error
	)
	if origin == uint64(0) {
		header, err = e.store.Head()
	} else {
		header, err = e.store.GetByHeight(context.Background(), origin)
	}
	if err != nil {
		log.Errorw("getting header by height", "height", origin, "err", err.Error())
		//nolint:errcheck
		stream.Reset()
		return
	}
	resp, err := ExtendedHeaderToProto(header)
	if err != nil {
		log.Errorw("marshaling header to proto", "height", origin, "err", err.Error())
		//nolint:errcheck
		stream.Reset()
		return
	}
	_, err = serde.Write(stream, resp)
	if err != nil {
		log.Errorw("writing header to stream", "height", origin, "err", err.Error())
	}
}

func (e *exchange) RequestHead(ctx context.Context) (*ExtendedHeader, error) {
	// create request
	req := &pb.ExtendedHeaderRequest{
		Origin: uint64(0),
		Amount: 1,
	}
	headers, err := e.performRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return headers[0], nil
}

func (e *exchange) RequestHeader(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	// sanity check height
	if height == 0 {
		return nil, fmt.Errorf("specified request height must be greater than 0")
	}
	// create request
	req := &pb.ExtendedHeaderRequest{
		Origin: height,
		Amount: 1,
	}
	headers, err := e.performRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return headers[0], nil
}

func (e *exchange) RequestHeaders(ctx context.Context, origin, amount uint64) ([]*ExtendedHeader, error) {
	// create request
	req := &pb.ExtendedHeaderRequest{
		Origin: origin,
		Amount: amount,
	}
	return e.performRequest(ctx, req)
}

func (e *exchange) performRequest(ctx context.Context, req *pb.ExtendedHeaderRequest) ([]*ExtendedHeader, error) {
	stream, err := e.host.NewStream(ctx, e.peer.ID, headerExchangeProtocolID)
	if err != nil {
		return nil, err
	}
	// send request
	_, err = serde.Write(stream, req)
	if err != nil {
		//nolint:errcheck
		stream.Reset()
		return nil, err
	}
	// read responses
	headers := make([]*ExtendedHeader, req.Amount)
	for i := 0; i < int(req.Amount); i++ {
		resp := new(pb.ExtendedHeader)
		_, err := serde.Read(stream, resp)
		if err != nil {
			//nolint:errcheck
			stream.Reset()
			return nil, err
		}
		header, err := ProtoToExtendedHeader(resp)
		if err != nil {
			//nolint:errcheck
			stream.Reset()
			return nil, err
		}
		headers[i] = header
	}
	// ensure at least one header was retrieved
	if len(headers) == 0 {
		return nil, fmt.Errorf("no headers found")
	}
	return headers, stream.Close()
}
