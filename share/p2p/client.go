package p2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
)

type ClientConfig struct {
	ReadTimeout, WriteTimeout time.Duration
	Interceptors              []ClientInterceptor
}

type Client struct {
	ctx                       context.Context
	host                      host.Host
	readTimeout, writeTimeout time.Duration
	interceptorsChain         ClientInterceptor
}

type DoFn func(context.Context, *Session) error

type ClientInterceptor func(context.Context, *Session, DoFn) error

func NewCLient(cfg ClientConfig, host host.Host) *Client {
	return &Client{
		host:              host,
		readTimeout:       cfg.ReadTimeout,
		writeTimeout:      cfg.WriteTimeout,
		interceptorsChain: chainClientInterceptors(cfg.Interceptors...),
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

	session := NewSession(stream, c.writeTimeout, c.readTimeout)
	return c.interceptorsChain(ctx, session, do)
}
