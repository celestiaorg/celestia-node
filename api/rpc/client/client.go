package client

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

//go:generate go run github.com/golang/mock/mockgen -destination=../../mocks/api.go -package=mocks . API
type API interface {
	fraud.Module
	header.Module
	state.Module
	share.Module
	das.Module
}

type Client struct {
	Fraud  fraud.API
	Header header.API
	State  state.API
	Share  share.API
	DAS    das.API
}

// MultiClientCloser is a wrapper struct to close clients across multiple namespaces.
type MultiClientCloser struct {
	closers map[string]jsonrpc.ClientCloser
}

// Register adds a new closer to the MultiClientCloser under the given namespace.
func (m *MultiClientCloser) Register(namespace string, closer jsonrpc.ClientCloser) {
	if m.closers == nil {
		m.closers = make(map[string]jsonrpc.ClientCloser)
	}
	m.closers[namespace] = closer
}

// CloseNamespace closes the client for the given namespace.
func (m *MultiClientCloser) CloseNamespace(namespace string) {
	m.closers[namespace]()
}

// CloseAll closes all saved clients.
func (m *MultiClientCloser) CloseAll() {
	for _, closer := range m.closers {
		closer()
	}
}

// NewClient creates a new Client with one connection per namespace.
func NewClient(ctx context.Context, addr string) (*Client, *MultiClientCloser, error) {
	var client Client
	var multiCloser MultiClientCloser

	// TODO: this duplication of strings many times across the codebase can be avoided with issue #1176
	var modules = map[string]interface{}{
		"share":  &client.Share,
		"state":  &client.State,
		"header": &client.Header,
		"fraud":  &client.Fraud,
		"das":    &client.DAS,
	}
	for name, module := range modules {
		closer, err := jsonrpc.NewClient(ctx, addr, name, module, nil)
		if err != nil {
			return nil, &multiCloser, err
		}
		multiCloser.Register(name, closer)
	}

	return &client, &multiCloser, nil
}
