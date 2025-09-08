package shrex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/libs/utils"
	shrexpb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/pb"
)

// Client implements client side of shrex protocol to obtain data from remote
// peers.
type Client struct {
	params *ClientParams

	host    host.Host
	metrics *Metrics
}

// NewClient creates a new shrEx client
func NewClient(params *ClientParams, host host.Host) (*Client, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex/client: parameters are not valid: %w", err)
	}
	return &Client{
		host:   host,
		params: params,
	}, nil
}

func (c *Client) WithMetrics() error {
	metrics, err := InitClientMetrics()
	if err != nil {
		return fmt.Errorf("shrex/client: init Metrics: %w", err)
	}
	c.metrics = metrics
	return nil
}

func (c *Client) Get(
	ctx context.Context,
	req request,
	resp response,
	peer peer.ID,
) error {
	logger := log.With(
		"source", "client",
		"name", req.Name(),
		"height", req.Height(),
		"peer", peer.String(),
	)
	requestTime := time.Now()
	status, err := c.doRequest(ctx, logger, req, resp, peer)
	if err != nil {
		logger.Warnw("requesting data from peer failed", "error", err)
	}
	c.metrics.observeRequests(ctx, 1, req.Name(), status, time.Since(requestTime))
	logger.Debugw("requested data", "status", status, "duration", time.Since(requestTime))
	return err
}

// doRequest performs a request to the given peer
// and expecting a response along with a payload that will be written into `container`.
func (c *Client) doRequest(
	ctx context.Context,
	logger *zap.SugaredLogger,
	req request,
	resp response,
	peer peer.ID,
) (status, error) {
	streamOpenCtx, cancel := context.WithTimeout(ctx, c.params.ReadTimeout)
	defer cancel()

	var err error
	ctx, span := tracer.Start(ctx, "shrex/client/request")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	stream, err := c.host.NewStream(streamOpenCtx, peer, ProtocolID(c.params.NetworkID(), req.Name()))
	if err != nil {
		return statusOpenStreamErr, fmt.Errorf("open stream: %w", err)
	}

	c.setStreamDeadlines(ctx, logger, stream)

	_, err = req.WriteTo(stream)
	if err != nil {
		return statusSendReqErr, fmt.Errorf("writing request: %w", err)
	}
	span.AddEvent("wrote request to stream")

	err = stream.CloseWrite()
	if err != nil {
		logger.Warnw("closing write side of the stream", "err", err)
	}

	var statusResp shrexpb.Response
	_, err = serde.Read(stream, &statusResp)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return statusRateLimited, fmt.Errorf("reading a response: %w", ErrRateLimited)
		}
		return statusReadStatusErr, fmt.Errorf("unexpected error during reading the status from stream: %w", err)
	}
	span.AddEvent("read status from stream")

	switch statusResp.Status {
	case shrexpb.Status_OK:
	case shrexpb.Status_NOT_FOUND:
		err = ErrNotFound
		return statusNotFound, err
	case shrexpb.Status_INTERNAL:
		err = ErrInternalServer
		return statusInternalErr, err
	default:
		err = ErrInvalidRequest
		return statusReadRespErr, err
	}

	_, err = resp.ReadFrom(stream)
	if err != nil {
		return statusReadRespErr, fmt.Errorf("%w: %w", ErrInvalidResponse, err)
	}

	span.AddEvent("read response from stream")
	return statusSuccess, nil
}

func (c *Client) setStreamDeadlines(ctx context.Context, logger *zap.SugaredLogger, stream network.Stream) {
	// set read/write deadline to use context deadline if it exists
	deadline, ok := ctx.Deadline()
	if ok {
		err := stream.SetDeadline(deadline)
		if err == nil {
			return
		}
		logger.Debugw("set stream deadline", "err", err)
	}

	// if deadline not set, client read deadline defaults to server write deadline
	if c.params.WriteTimeout != 0 {
		err := stream.SetReadDeadline(time.Now().Add(c.params.WriteTimeout))
		if err != nil {
			logger.Debugw("set read deadline", "err", err)
		}
	}

	// if deadline not set, client write deadline defaults to server read deadline
	if c.params.ReadTimeout != 0 {
		err := stream.SetWriteDeadline(time.Now().Add(c.params.ReadTimeout))
		if err != nil {
			logger.Debugw("set write deadline", "err", err)
		}
	}
}
