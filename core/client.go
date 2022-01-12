package core

import (
	"fmt"

	retryhttp "github.com/hashicorp/go-retryablehttp"

	corenode "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/rpc/client/local"
)

// Client is an alias to Core Client.
type Client = client.Client

// NewRemote creates a new Client that communicates with a remote Core endpoint over HTTP.
func NewRemote(protocol, remoteAddr string) (Client, error) {
	httpClient := retryhttp.NewClient()
	httpClient.RetryMax = 3

	return http.NewWithClient(
		fmt.Sprintf("%s://%s", protocol, remoteAddr),
		"/websocket",
		httpClient.StandardClient(),
	)
}

// NewEmbedded returns a new Client from an embedded Core node process.
func NewEmbedded(cfg *Config) (Client, error) {
	node, err := corenode.DefaultNewNode(cfg, adaptedLogger())
	if err != nil {
		return nil, err
	}

	return &embeddedWrapper{local.New(node), node}, nil
}

// NewEmbeddedFromNode wraps a given Core node process to be able to control its lifecycle.
func NewEmbeddedFromNode(node *corenode.Node) Client {
	return &embeddedWrapper{local.New(node), node}
}

// embeddedWrapper is a small wrapper around local Client which ensures the embedded Core node
// can be started/stopped.
type embeddedWrapper struct {
	*local.Local
	node *corenode.Node
}

func (e *embeddedWrapper) Start() error {
	return e.node.Start()
}

func (e *embeddedWrapper) Stop() error {
	return e.node.Stop()
}
