package internal

import (
	"context"
	"fmt"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"strings"
	"time"
)

// NewCoreConn creates a new gRPC client connection with retry interceptors and connectivity checks.
func NewCoreConn(endpoint string) (*grpc.ClientConn, error) {
	retryInterceptor := grpc_retry.UnaryClientInterceptor(
		grpc_retry.WithMax(5),
		grpc_retry.WithCodes(codes.Unavailable),
		grpc_retry.WithBackoff(
			grpc_retry.BackoffExponentialWithJitter(time.Second, 2.0)),
	)
	retryStreamInterceptor := grpc_retry.StreamClientInterceptor(
		grpc_retry.WithMax(5),
		grpc_retry.WithCodes(codes.Unavailable),
		grpc_retry.WithBackoff(
			grpc_retry.BackoffExponentialWithJitter(time.Second, 2.0)),
	)

	opts := []grpc.DialOption{
		grpc.WithUnaryInterceptor(retryInterceptor),
		grpc.WithStreamInterceptor(retryStreamInterceptor),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	client, err := grpc.NewClient(strings.TrimPrefix(endpoint, "tcp://"), opts...)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	if ready := client.WaitForStateChange(ctx, connectivity.Ready); !ready {
		return nil, fmt.Errorf("client is not ready")
	}
	return client, nil
}
