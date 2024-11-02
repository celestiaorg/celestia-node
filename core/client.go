package core

import (
	"fmt"
	coregrpc "github.com/tendermint/tendermint/rpc/grpc"
)

// Client is an alias to Core Client.
type Client = coregrpc.BlockAPIClient

// NewRemote creates a new Client that communicates with a remote Core endpoint over gRPC.
func NewRemote(ip, port string) (Client, error) {
	return coregrpc.StartBlockAPIGRPCClient(fmt.Sprintf("tcp://%s:%s", ip, port))
}
