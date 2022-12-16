package p2p

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/header"
	p2p_pb "github.com/celestiaorg/celestia-node/header/p2p/pb"
)

func protocolID(protocolSuffix string) protocol.ID {
	return protocol.ID(fmt.Sprintf("/header-ex/v0.0.3/%s", protocolSuffix))
}

// sendMessage opens the stream to the given peers and sends ExtendedHeaderRequest to fetch
// ExtendedHeaders. As a result sendMessage returns ExtendedHeaderResponse, the size of fetched
// data, the duration of the request and an error.
func sendMessage(
	ctx context.Context,
	host host.Host,
	to peer.ID,
	protocol protocol.ID,
	req *p2p_pb.ExtendedHeaderRequest,
) ([]*p2p_pb.ExtendedHeaderResponse, uint64, uint64, error) {
	startTime := time.Now()
	stream, err := host.NewStream(ctx, to, protocol)
	if err != nil {
		return nil, 0, 0, err
	}

	// set stream deadline from the context deadline.
	// if it is empty, then we assume that it will
	// hang until the server will close the stream by the timeout.
	if dl, ok := ctx.Deadline(); ok {
		if err = stream.SetDeadline(dl); err != nil {
			log.Debugw("error setting deadline: %s", err)
		}
	}

	// send request
	_, err = serde.Write(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, 0, 0, err
	}

	err = stream.CloseWrite()
	if err != nil {
		return nil, 0, 0, nil
	}

	headers := make([]*p2p_pb.ExtendedHeaderResponse, 0)
	totalRequestSize := uint64(0)
	for i := 0; i < int(req.Amount); i++ {
		resp := new(p2p_pb.ExtendedHeaderResponse)
		msgSize, err := serde.Read(stream, resp)
		if err != nil {
			if err == io.EOF {
				break
			}
			stream.Reset() //nolint:errcheck
			return nil, 0, 0, err
		}

		totalRequestSize += uint64(msgSize)
		headers = append(headers, resp)
	}

	duration := time.Since(startTime).Milliseconds()
	if err = stream.Close(); err != nil {
		log.Errorw("closing stream", "err", err)
	}

	return headers, totalRequestSize, uint64(duration), nil
}

// convertStatusCodeToError converts passed status code into an error.
func convertStatusCodeToError(code p2p_pb.StatusCode) error {
	switch code {
	case p2p_pb.StatusCode_OK:
		return nil
	case p2p_pb.StatusCode_NOT_FOUND:
		return header.ErrNotFound
	case p2p_pb.StatusCode_LIMIT_EXCEEDED:
		return header.ErrHeadersLimitExceeded
	default:
		return fmt.Errorf("unknown status code %d", code)
	}
}
