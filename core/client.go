package core

import (
	"fmt"

	retryhttp "github.com/hashicorp/go-retryablehttp"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/http"
)

// Client is an alias to Core Client.
type Client = client.Client

// NewRemote creates a new Client that communicates with a remote Core endpoint over HTTP.
func NewRemote(ip, port string) (Client, error) {
	httpClient := retryhttp.NewClient()
	httpClient.RetryMax = 2
	// suppress logging
	httpClient.Logger = nil

	return http.NewWithClient(
		fmt.Sprintf("tcp://%s:%s", ip, port),
		"/websocket",
		httpClient.StandardClient(),
	)
}
