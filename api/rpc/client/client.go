package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// TODO: this duplication of strings many times across the codebase can be avoided with issue #1176
var client Client
var Modules = map[string]interface{}{
	"share":  &client.Share.Internal,
	"state":  &client.State.Internal,
	"header": &client.Header.Internal,
	"fraud":  &client.Fraud.Internal,
	"das":    &client.DAS.Internal,
	"p2p":    &client.P2P.Internal,
	"node":   &client.Node.Internal,
}

type Client struct {
	Fraud  fraud.API
	Header header.API
	State  state.API
	Share  share.API
	DAS    das.API
	P2P    p2p.API
	Node   node.API

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

// NewPublicClient creates a new Client with one connection per namespace.
func NewPublicClient(ctx context.Context, addr string) (*Client, error) {
	return newClient(ctx, addr, nil)
}

// NewClient creates a new Client with one connection per namespace with the
// given token as the authorization token.
func NewClient(ctx context.Context, addr string, token string) (*Client, error) {
	authHeader := http.Header{perms.AuthKey: []string{fmt.Sprintf("Bearer %s", token)}}
	return newClient(ctx, addr, authHeader)
}

func newClient(ctx context.Context, addr string, authHeader http.Header) (*Client, error) {
	var multiCloser multiClientCloser
	for name, module := range Modules {
		closer, err := jsonrpc.NewClient(ctx, addr, name, module, authHeader)
		if err != nil {
			return nil, err
		}
		multiCloser.register(closer)
	}

	return &client, nil
}
