package p2p

import (
	"context"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"time"
)

type ClientConfig struct {
	readTimeout, writeTimeout time.Duration
	interceptors              []ClientInterceptor
}

type Client struct {
	ctx                       context.Context
	host                      host.Host
	readTimeout, writeTimeout time.Duration
	interceptorsChain         ClientInterceptor
}

type DoFn func(context.Context, *Session) error

type ClientInterceptor func(context.Context, *Session, DoFn) error

func NewCLient(ctx context.Context, cfg ClientConfig, host host.Host) *Client {
	return &Client{
		ctx:               ctx,
		host:              host,
		readTimeout:       cfg.readTimeout,
		writeTimeout:      cfg.writeTimeout,
		interceptorsChain: chainClientInterceptors(cfg.interceptors...),
	}
}

func chainClientInterceptors(interceptors ...ClientInterceptor) ClientInterceptor {
	n := len(interceptors)
	return func(ctx context.Context, session *Session, do DoFn) error {
		chainer := func(currentInter ClientInterceptor, currentRequestor DoFn) DoFn {
			return func(ctx context.Context, s *Session) error {
				return currentInter(ctx, s, currentRequestor)
			}
		}

		chainedHandler := do
		for i := n - 1; i >= 0; i-- {
			chainedHandler = chainer(interceptors[i], chainedHandler)
		}

		return chainedHandler(ctx, session)
	}
}

func (c *Client) Do(ctx context.Context, peer peer.ID, protocol protocol.ID, do DoFn) error {
	stream, err := c.host.NewStream(ctx, peer, protocol)
	if err != nil {
		return err
	}
	defer stream.Close()

	session := NewSession(ctx, stream, c.writeTimeout, c.readTimeout)
	return c.interceptorsChain(ctx, session, do)
}
