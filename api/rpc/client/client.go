package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/blobstream"
	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/modname"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

var (
	// staticClient is used for generating the OpenRPC spec.
	staticClient Client
	Modules      = moduleMap(&staticClient)
)

type Client struct {
	Fraud      fraud.API
	Header     header.API
	State      state.API
	Share      share.API
	DAS        das.API
	P2P        p2p.API
	Node       node.API
	Blob       blob.API
	DA         da.API
	Blobstream blobstream.API

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

// Close closes the connections to all namespaces registered on the staticClient.
func (c *Client) Close() {
	c.closer.closeAll()
}

// NewClient creates a new Client with one connection per namespace with the
// given token as the authorization token.
func NewClient(ctx context.Context, addr, token string) (*Client, error) {
	var authHeader http.Header
	if token != "" {
		authHeader = http.Header{perms.AuthKey: []string{fmt.Sprintf("Bearer %s", token)}}
	}
	return newClient(ctx, addr, authHeader)
}

func newClient(ctx context.Context, addr string, authHeader http.Header) (*Client, error) {
	var multiCloser multiClientCloser
	var client Client
	for name, module := range moduleMap(&client) {
		closer, err := jsonrpc.NewClient(ctx, addr, name, module, authHeader)
		if err != nil {
			return nil, err
		}
		multiCloser.register(closer)
	}

	return &client, nil
}

func moduleMap(client *Client) map[string]any {
	return map[string]any{
		modname.Share:      &client.Share.Internal,
		modname.State:      &client.State.Internal,
		modname.Header:     &client.Header.Internal,
		modname.Fraud:      &client.Fraud.Internal,
		modname.DAS:        &client.DAS.Internal,
		modname.P2P:        &client.P2P.Internal,
		modname.Node:       &client.Node.Internal,
		modname.Blob:       &client.Blob.Internal,
		modname.DA:         &client.DA.Internal,
		modname.Blobstream: &client.Blobstream.Internal,
	}
}
