package core

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

func NewGRPCClient(ctx context.Context, addr string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, err
	}

	conn.Connect()
	if !conn.WaitForStateChange(ctx, connectivity.Ready) {
		return nil, errors.New("couldn't connect to core endpoint")
	}
	return conn, nil
}
