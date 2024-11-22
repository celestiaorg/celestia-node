package core

import (
	"fmt"

	coregrpc "github.com/tendermint/tendermint/rpc/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a core gRPC client.
type Client struct {
	coregrpc.BlockAPIClient
	host, port string
	conn       *grpc.ClientConn
}

// NewClient creates a new Client that communicates with a remote Core endpoint over gRPC.
// The connection is not started when creating the client.
// Use the Start method to start the connection.
func NewClient(host, port string) *Client {
	return &Client{
		host: host,
		port: port,
	}
}

// Start created the Client's gRPC connection with optional dial options.
// If the connection is already started, it does nothing.
func (c *Client) Start(opts ...grpc.DialOption) error {
	if c.IsRunning() {
		return nil
	}
	if len(opts) == 0 {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%s", c.host, c.port),
		opts...,
	)
	if err != nil {
		return err
	}
	c.conn = conn

	c.BlockAPIClient = coregrpc.NewBlockAPIClient(conn)
	return nil
}

// IsRunning checks if the client's connection is established and ready for use.
// It returns true if the connection is active, false otherwise.
func (c *Client) IsRunning() bool {
	return c.conn != nil && c.BlockAPIClient != nil
}

// Stop terminates the Client's gRPC connection and releases all related resources.
// If the connection is already stopped, it does nothing.
func (c *Client) Stop() error {
	if !c.IsRunning() {
		return nil
	}
	defer func() {
		c.conn = nil
		c.BlockAPIClient = nil
	}()
	return c.conn.Close()
}
