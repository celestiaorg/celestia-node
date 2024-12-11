package core

import (
	"google.golang.org/grpc"
)

func NewGRPCClient(addr string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
