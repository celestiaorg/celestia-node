package client

import (
	"context"
	"crypto/tls"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

// GRPCConfig combines all configuration fields for managing the relationship with a Core node.
type GRPCConfig struct {
	Addr string
	// TLSEnabled specifies whether the connection is secure or not.
	TLSEnabled bool
	// AuthToken is the authentication token to be used for gRPC authentication.
	// If left empty, the client will not include the authentication token in its requests.
	// Note: AuthToken is insecure without TLS
	AuthToken string
}

// Validate performs basic validation of the config.
func (cfg *GRPCConfig) Validate() error {
	_, err := utils.SanitizeAddr(cfg.Addr)
	return err
}

func grpcClient(cfg GRPCConfig) (*grpc.ClientConn, error) {
	const defaultGRPCMessageSize = 64 * 1024 * 1024 // 64Mb
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(defaultGRPCMessageSize),
			grpc.MaxCallSendMsgSize(defaultGRPCMessageSize),
		),
	}
	if cfg.TLSEnabled {
		opts = append(opts, grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})),
		)
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

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

	opts = append(opts,
		grpc.WithUnaryInterceptor(retryInterceptor),
		grpc.WithStreamInterceptor(retryStreamInterceptor),
	)

	if cfg.AuthToken != "" {
		if !cfg.TLSEnabled {
			log.Warn("auth token is set but TLS is disabled, this is insecure")
		}
		opts = append(opts,
			grpc.WithChainUnaryInterceptor(authInterceptor(cfg.AuthToken), retryInterceptor),
			grpc.WithChainStreamInterceptor(authStreamInterceptor(cfg.AuthToken), retryStreamInterceptor),
		)
	}
	return grpc.NewClient(cfg.Addr, opts...)
}

func authInterceptor(xtoken string) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-token", xtoken)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func authStreamInterceptor(xtoken string) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-token", xtoken)
		return streamer(ctx, desc, cc, method, opts...)
	}
}
