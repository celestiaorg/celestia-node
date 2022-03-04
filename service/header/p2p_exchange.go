package header

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	pb "github.com/celestiaorg/celestia-node/service/header/pb"
)

var exchangeProtocolID = protocol.ID("/header-ex/v0.0.1")

// P2PExchange enables sending outbound ExtendedHeaderRequests to the network as well as
// handling inbound ExtendedHeaderRequests from the network.
type P2PExchange struct {
	host host.Host

	trustedPeers peer.IDSlice
}

func NewP2PExchange(host host.Host, peers peer.IDSlice) *P2PExchange {
	return &P2PExchange{
		host:         host,
		trustedPeers: peers,
	}
}

func (ex *P2PExchange) RequestHead(ctx context.Context) (*ExtendedHeader, error) {
	log.Debug("p2p: requesting head")
	// create request
	req := &pb.ExtendedHeaderRequest{
		Origin: uint64(0),
		Amount: 1,
	}
	headers, err := ex.performRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return headers[0], nil
}

func (ex *P2PExchange) RequestHeader(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	log.Debugw("p2p: requesting header", "height", height)
	// sanity check height
	if height == 0 {
		return nil, fmt.Errorf("specified request height must be greater than 0")
	}
	// create request
	req := &pb.ExtendedHeaderRequest{
		Origin: height,
		Amount: 1,
	}
	headers, err := ex.performRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return headers[0], nil
}

func (ex *P2PExchange) RequestHeaders(ctx context.Context, from, amount uint64) ([]*ExtendedHeader, error) {
	log.Debugw("p2p: requesting headers", "from", from, "to", from+amount)
	// create request
	req := &pb.ExtendedHeaderRequest{
		Origin: from,
		Amount: amount,
	}
	return ex.performRequest(ctx, req)
}

func (ex *P2PExchange) RequestByHash(ctx context.Context, hash tmbytes.HexBytes) (*ExtendedHeader, error) {
	log.Debugw("p2p: requesting header", "hash", hash.String())
	// create request
	req := &pb.ExtendedHeaderRequest{
		Hash:   hash.Bytes(),
		Amount: 1,
	}
	headers, err := ex.performRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(headers[0].Hash().Bytes(), hash) {
		return nil, fmt.Errorf("incorrect hash in header: expected %x, got %x", hash, headers[0].Hash().Bytes())
	}
	return headers[0], nil
}

func (ex *P2PExchange) performRequest(ctx context.Context, req *pb.ExtendedHeaderRequest) ([]*ExtendedHeader, error) {
	if req.Amount == 0 {
		return make([]*ExtendedHeader, 0), nil
	}

	if len(ex.trustedPeers) == 0 {
		return nil, fmt.Errorf("no trusted peers")
	}

	// nolint:gosec // G404: Use of weak random number generator
	index := rand.Intn(len(ex.trustedPeers))
	stream, err := ex.host.NewStream(ctx, ex.trustedPeers[index], exchangeProtocolID)
	if err != nil {
		return nil, err
	}
	// send request
	_, err = serde.Write(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, err
	}
	// read responses
	headers := make([]*ExtendedHeader, req.Amount)
	for i := 0; i < int(req.Amount); i++ {
		resp := new(pb.ExtendedHeader)
		_, err := serde.Read(stream, resp)
		if err != nil {
			stream.Reset() //nolint:errcheck
			return nil, err
		}

		header, err := ProtoToExtendedHeader(resp)
		if err != nil {
			stream.Reset() //nolint:errcheck
			return nil, err
		}

		headers[i] = header
	}
	// ensure at least one header was retrieved
	if len(headers) == 0 {
		return nil, ErrNotFound
	}
	return headers, stream.Close()
}
