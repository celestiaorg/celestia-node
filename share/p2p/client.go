package p2p

import (
	"context"
	"errors"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
)

var errNoPeers = errors.New("no peers provided")

type ClientConfig struct {
	ReadTimeout, WriteTimeout time.Duration
	Interceptors              []ClientInterceptor
}

// Client is a utility wrapper around libp2p host to perform client side calls.
type Client struct {
	ctx                       context.Context
	host                      host.Host
	readTimeout, writeTimeout time.Duration
	interceptorsChain         ClientInterceptor
}

// DoFn handles client side of communication
type DoFn func(context.Context, *Session) error

// performFn performs clients request against given protocol and peers
type performFn func(ctx context.Context, protocol protocol.ID, peers ...peer.ID) error

// ClientInterceptor is the client side middleware
type ClientInterceptor func(context.Context, performFn, protocol.ID, ...peer.ID) error

func NewCLient(cfg ClientConfig, host host.Host) *Client {
	return &Client{
		host:              host,
		readTimeout:       cfg.ReadTimeout,
		writeTimeout:      cfg.WriteTimeout,
		interceptorsChain: chainClientInterceptors(cfg.Interceptors...),
	}
}

// chainClientInterceptors reduces multiple interceptors to one chain. Execution order of
// interceptors is from last to first.
func chainClientInterceptors(interceptors ...ClientInterceptor) ClientInterceptor {
	n := len(interceptors)
	return func(ctx context.Context, requester performFn, pid protocol.ID, peers ...peer.ID) error {
		chainer := func(currentInter ClientInterceptor, currentRequester performFn) performFn {
			return func(ctx context.Context, pid protocol.ID, peers ...peer.ID) error {
				return currentInter(ctx, currentRequester, pid, peers...)
			}
		}

		chainedRequester := requester
		for i := n - 1; i >= 0; i-- {
			chainedRequester = chainer(interceptors[i], chainedRequester)
		}

		return chainedRequester(ctx, pid, peers...)
	}
}

// Do executes clients request
func (c *Client) Do(ctx context.Context, do DoFn, pid protocol.ID, peers ...peer.ID) error {
	perform := func(ctx context.Context, protocol protocol.ID, peers ...peer.ID) error {
		if len(peers) == 0 {
			return errNoPeers
		}
		stream, err := c.host.NewStream(ctx, peers[0], pid)
		if err != nil {
			return err
		}
		defer stream.Close()

		session := NewSession(stream, c.writeTimeout, c.readTimeout)
		err = do(ctx, session)
		if err != nil {
			stream.Reset() //nolint:errcheck
			return err
		}

		return stream.Close()
	}

	return c.interceptorsChain(ctx, perform, pid, peers...)
}
