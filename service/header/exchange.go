package header

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
)

var headerExchangeProtocolID = protocol.ID("expected-exchange")

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
	buf := make([]byte, 100) // TODO @renaynay: use Messenger lib to determine size of packet before reading. https://github.com/celestiaorg/go-libp2p-messenger/tree/main/serde
	reqSize, err := stream.Read(buf)
	if err != nil {
		log.Errorw("reading header request from stream", "err", err.Error())
		//nolint:errcheck
		stream.Reset()
		return
	}
	request := new(ExtendedHeaderRequest)
	err = request.UnmarshalBinary(buf[:reqSize])
	if err != nil {
		log.Errorw("unmarshaling inbound header request", "err", err.Error())
		//nolint:errcheck
		stream.Reset()
		return
	}
	// route depending on amount of headers requested
	if request.Amount > 1 {
		e.handleMultipleHeaderRequest(request, stream)
	} else {
		e.handleSingleHeaderRequest(request.Origin, stream) // TODO @renaynay: should we parallelise this?
	}
}

// handleSingleHeaderRequest handles an ExtendedHeaderRequest for a single header and writes
// the header to the given stream.
func (e *exchange) handleSingleHeaderRequest(origin uint64, stream network.Stream) {
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
	bin, err := header.MarshalBinary()
	if err != nil {
		log.Errorw("marshaling header", "height", origin, "err", err.Error())
		//nolint:errcheck
		stream.Reset()
		return
	}
	_, err = stream.Write(bin) // TODO @renaynay: use serde.Write from the golibp2p-messenger lib
	if err != nil {
		log.Errorw("writing header to stream", "height", origin, "err", err.Error())
	}
}

// handleMultipleHeaderRequest handles responding to an ExtendedHeaderRequest for
// multiple headers.
func (e *exchange) handleMultipleHeaderRequest(request *ExtendedHeaderRequest, stream network.Stream) {
	height := request.Origin
	amount := uint64(0)
	for amount < request.Amount {
		e.handleSingleHeaderRequest(height, stream)
		height++
		amount++
	}
}

func (e *exchange) RequestHead(ctx context.Context) (*ExtendedHeader, error) {
	stream, err := e.host.NewStream(ctx, e.peer.ID, headerExchangeProtocolID)
	if err != nil {
		return nil, err
	}
	// create request
	req := &ExtendedHeaderRequest{
		Origin: uint64(0),
		Amount: 1,
	}
	bin, err := req.MarshalBinary()
	if err != nil {
		return nil, err
	}
	// send request
	_, err = stream.Write(bin)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 2000) // TODO @renaynay: how big do we expect ExtendedHeader to be?
	msgSize, err := stream.Read(buf)
	if err != nil {
		return nil, err
	}
	// unmarshal response
	resp := new(ExtendedHeader)
	err = resp.UnmarshalBinary(buf[:msgSize])
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (e *exchange) RequestHeader(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	// sanity check height
	if height == 0 {
		return nil, fmt.Errorf("specified request height must be greater than 0")
	}
	stream, err := e.host.NewStream(ctx, e.peer.ID, headerExchangeProtocolID)
	if err != nil {
		return nil, err
	}
	// create request
	req := &ExtendedHeaderRequest{
		Origin: height,
		Amount: 1,
	}
	bin, err := req.MarshalBinary()
	if err != nil {
		return nil, err
	}
	// send request
	_, err = stream.Write(bin)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 2000) // TODO @renaynay: how big do we expect ExtendedHeader to be?
	msgSize, err := stream.Read(buf)
	if err != nil {
		return nil, err
	}
	// unmarshal response
	resp := new(ExtendedHeader)
	err = resp.UnmarshalBinary(buf[:msgSize])
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (e *exchange) RequestHeaders(ctx context.Context, origin, amount uint64) ([]*ExtendedHeader, error) {
	stream, err := e.host.NewStream(ctx, e.peer.ID, headerExchangeProtocolID)
	if err != nil {
		return nil, err
	}
	// create request
	req := &ExtendedHeaderRequest{
		Origin: origin,
		Amount: amount,
	}
	bin, err := req.MarshalBinary()
	if err != nil {
		return nil, err
	}
	// send request
	_, err = stream.Write(bin)
	if err != nil {
		return nil, err
	}
	// read responses
	resp := make([]*ExtendedHeader, amount)
	for i := 0; i < int(amount); i++ {
		buf := make([]byte, 2000) // TODO @renaynay: use serde
		msgSize, err := stream.Read(buf)
		if err != nil {
			return nil, err
		}
		// unmarshal response
		eh := new(ExtendedHeader)
		err = eh.UnmarshalBinary(buf[:msgSize])
		if err != nil {
			return nil, err
		}
		resp[i] = eh
	}
	return resp, nil
}
