package p2p

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/libs/header"
	p2p_pb "github.com/celestiaorg/celestia-node/libs/header/p2p/pb"
)

func protocolID(networkID string) protocol.ID {
	return protocol.ID(fmt.Sprintf("/%s/header-ex/v0.0.3", networkID))
}

func PubsubTopicID(networkID string) string {
	return fmt.Sprintf("/%s/header-sub/v0.0.1", networkID)
}

func validateChainID(want, have string) error {
	if want != "" && !strings.EqualFold(want, have) {
		return fmt.Errorf("header with different chainID received.want=%s,have=%s",
			want, have,
		)
	}
	return nil
}

// sendMessage opens the stream to the given peers and sends HeaderRequest to fetch
// Headers. As a result sendMessage returns HeaderResponse, the size of fetched
// data, the duration of the request and an error.
func sendMessage(
	ctx context.Context,
	host host.Host,
	to peer.ID,
	protocol protocol.ID,
	req *p2p_pb.HeaderRequest,
) ([]*p2p_pb.HeaderResponse, uint64, uint64, error) {
	startTime := time.Now()
	stream, err := host.NewStream(ctx, to, protocol)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("header/p2p: failed to open a new stream: %w", err)
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
		return nil, 0, 0, fmt.Errorf("header/p2p: failed to write a request: %w", err)
	}

	err = stream.CloseWrite()
	if err != nil {
		return nil, 0, 0, err
	}

	headers := make([]*p2p_pb.HeaderResponse, 0)

	var totalRespLn uint64
	for i := 0; i < int(req.Amount); i++ {
		resp := new(p2p_pb.HeaderResponse)
		respLn, readErr := serde.Read(stream, resp)
		if readErr != nil {
			err = readErr
			break
		}

		totalRespLn += uint64(respLn)
		headers = append(headers, resp)
	}

	duration := time.Since(startTime).Milliseconds()

	// we allow the server side to explicitly close the connection
	// if it does not have the requested range.
	// In this case, server side will send us a response with ErrNotFound status code inside
	// and then will close the stream.
	// If the server side will have a part of the requested range, then it will send this part
	// and then will close the connection
	if err == io.EOF {
		err = nil
	}

	if err == nil {
		if closeErr := stream.Close(); closeErr != nil {
			log.Errorw("closing stream", "err", closeErr)
		}
	} else {
		// reset stream in case of an error
		stream.Reset() //nolint:errcheck
	}
	return headers, totalRespLn, uint64(duration), err
}

// convertStatusCodeToError converts passed status code into an error.
func convertStatusCodeToError(code p2p_pb.StatusCode) error {
	switch code {
	case p2p_pb.StatusCode_OK:
		return nil
	case p2p_pb.StatusCode_NOT_FOUND:
		return header.ErrNotFound
	default:
		return fmt.Errorf("unknown status code %d", code)
	}
}
