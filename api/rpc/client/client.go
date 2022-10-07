package client

import (
	"context"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/celestiaorg/celestia-node/nodebuilder/daser"
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
	daser.Module
}

type Client struct {
	Fraud  fraud.API
	Header header.API
	State  state.API
	Share  share.API
	DAS    daser.API
}

func NewClient(ctx context.Context, addr string) (*Client, jsonrpc.ClientCloser, error) {
	var client Client
	closer, err := jsonrpc.NewMergeClient(
		ctx,
		addr,
		"Handler",
		[]interface{}{&client.Share, &client.State, &client.Header, &client.Fraud, &client.DAS},
		nil,
	)
	return &client, closer, err
}
