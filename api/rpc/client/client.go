package client

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

type API interface {
	fraud.Module
	header.Module
	state.Module
	share.Module
	das.Module
	p2p.Module
}

type Client struct {
	Fraud  fraud.API
	Header header.API
	State  state.API
	Share  share.API
	DAS    das.API
	P2P    p2p.API

	closer multiClientCloser
}

// multiClientCloser is a wrapper struct to close clients across multiple namespaces.
type multiClientCloser struct {
	closers []jsonrpc.ClientCloser
}

// register adds a new closer to the multiClientCloser
func (m *multiClientCloser) register(closer jsonrpc.ClientCloser) {
	m.closers = append(m.closers, closer)
}

// closeAll closes all saved clients.
func (m *multiClientCloser) closeAll() {
	for _, closer := range m.closers {
		closer()
	}
}

// Close closes the connections to all namespaces registered on the client.
func (c *Client) Close() {
	c.closer.closeAll()
}

// NewClient creates a new Client with one connection per namespace.
func NewClient(ctx context.Context, addr string) (*Client, error) {
	var client Client
	var multiCloser multiClientCloser

	// TODO: this duplication of strings many times across the codebase can be avoided with issue #1176
	var modules = map[string]interface{}{
		"share":  &client.Share,
		"state":  &client.State,
		"header": &client.Header,
		"fraud":  &client.Fraud,
		"das":    &client.DAS,
		"p2p":    &client.P2P,
	}
	// TODO @distractedm1nd @renaynay: how does client know if daser is nil?
	for name, module := range modules {
		closer, err := jsonrpc.NewClient(ctx, addr, name, module, nil)
		if err != nil {
			return nil, err
		}
		multiCloser.register(closer)
	}

	return &client, nil
}
