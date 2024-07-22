package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/blobstream"
	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
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

type RPCClientOption func(*clientConfig)

type clientConfig struct {
	timeout time.Duration
}

func WithTimeout(timeout time.Duration) RPCClientOption {
	return func(c *clientConfig) {
		c.timeout = timeout
	}
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
func NewClient(ctx context.Context, addr, token string, opts ...RPCClientOption) (*Client, error) {
	config := &clientConfig{}
	for _, opt := range opts {
		opt(config)
	}

	authHeader := http.Header{perms.AuthKey: []string{fmt.Sprintf("Bearer %s", token)}}
	return newClient(ctx, addr, authHeader, config)
}

func newClient(ctx context.Context, addr string, authHeader http.Header, config *clientConfig) (*Client, error) {
	var client Client
	var multiCloser multiClientCloser

	httpClient := &http.Client{
		Timeout: config.timeout,
	}

	for name, module := range moduleMap(&client) {
		closer, err := jsonrpc.NewMergeClient(
			ctx,
			addr,
			name,
			[]interface{}{module},
			authHeader,
			jsonrpc.WithHTTPClient(httpClient),
		)
		if err != nil {
			return nil, err
		}
		multiCloser.register(closer)
	}

	client.closer = multiCloser
	return &client, nil
}

func moduleMap(client *Client) map[string]interface{} {
	// TODO: this duplication of strings many times across the codebase can be avoided with issue #1176
	return map[string]interface{}{
		"share":      &client.Share.Internal,
		"state":      &client.State.Internal,
		"header":     &client.Header.Internal,
		"fraud":      &client.Fraud.Internal,
		"das":        &client.DAS.Internal,
		"p2p":        &client.P2P.Internal,
		"node":       &client.Node.Internal,
		"blob":       &client.Blob.Internal,
		"da":         &client.DA.Internal,
		"blobstream": &client.Blobstream.Internal,
	}
}
