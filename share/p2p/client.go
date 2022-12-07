package p2p

import (
	"context"
	"errors"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"time"
)

const (
	connected state = iota
	disconnected
	idle
)

type Client struct {
	ctx                       context.Context
	host                      host.Host
	readTimeout, writeTimeout time.Duration
	resolver                  Resolver
	balancer                  Balancer
	interceptorsChain         Interceptor
}

// Resolver runs discovery of new peers and notifies balancer when connectivity changes happens
type Resolver interface {
	Run(Balancer)
	Subscribe(id protocol.ID)
	Stop(context.Context)
}

// Balancer maintains ready for use connection and provides logic for connection selection
type Balancer interface {
	// Update is called by Resolver when connectivity changes
	Update(state connState)
	// Pick selects peer for communication. If no connection is ready returns errConnNotReady
	Pick(protocol.ID) peer.ID
	// Score updates peers score
	Score(pid protocol.ID, peer peer.ID, score int64)
}

var errConnNotReady = errors.New("no ready connection")

type state int

type connState struct {
	peer  peer.ID
	state state
}

type Requestor interface {
	ProtocolID() protocol.ID
	Do(*Session) error
}

func newCLient(ctx context.Context, host host.Host) {}

func (c *Client) Subscribe(ids ...protocol.ID) error {
	for _, id := range ids {
		c.resolver.Subscribe(id)
	}
	return nil
}

func (c *Client) Do(ctx context.Context, req Requestor) error {
	pid := req.ProtocolID()
	peer := c.balancer.Pick(pid)

	stream, err := c.host.NewStream(ctx, peer, pid)
	if err != nil {
		return err
	}

	session := NewSession(ctx, stream, c.writeTimeout, c.readTimeout)
	return c.interceptorsChain(session, req.Do)
}
