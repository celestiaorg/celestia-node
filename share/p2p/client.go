package p2p

import (
	"context"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"time"
)

type Client struct {
	ctx                       context.Context
	host                      host.Host
	readTimeout, writeTimeout time.Duration
	connManager               peerTracker
	interceptorsChain         Interceptor
}

type peerTracker interface {
	Get(protocol.ID) peer.ID
	Add(protocol.ID, peer.ID)
	Remove(protocol.ID, peer.ID)
	Score(protocol.ID, peer.ID, int64)
}

func newCLient(ctx context.Context, host host.Host) {}

func (c *Client) Subscribe(id protocol.ID) error {
	connected, err := c.host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	if err != nil {
		return err
	}

	changed, err := c.host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	if err != nil {
		return err
	}

	go func(ctx context.Context) {
		defer connected.Close()
		defer changed.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case e := <-connected.Out():
				connStatus := e.(event.EvtPeerIdentificationCompleted)
				c.connManager.Add(id, connStatus.Peer)
			case e := <-changed.Out():
				connStatus := e.(event.EvtPeerConnectednessChanged)
				if connStatus.Connectedness == network.NotConnected {
					c.connManager.Remove(id, connStatus.Peer)
				}
			}
		}
	}(c.ctx)

	return nil
}

func (c *Client) Do(ctx context.Context, pid protocol.ID) (*Session, error) {
	peer := c.connManager.Get(pid)

	stream, err := c.host.NewStream(ctx, peer, pid)
	if err != nil {
		return nil, err
	}

	return NewSession(ctx, stream, c.writeTimeout, c.readTimeout), nil
}
