package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

// MultiGRPCClient manages multiple gRPC connections with failover support
type MultiGRPCClient struct {
	connections []*grpc.ClientConn
	configs     []CoreGRPCConfig
}

// NewMultiGRPCClient creates a new multi-endpoint gRPC client
func NewMultiGRPCClient(cfg CoreGRPCConfig) (*MultiGRPCClient, error) {
	// Collect all configurations (primary + additional)
	configs := []CoreGRPCConfig{cfg}
	configs = append(configs, cfg.AdditionalCoreGRPCConfigs...)
	
	// Create connections for all configurations
	connections := make([]*grpc.ClientConn, 0, len(configs))
	for _, config := range configs {
		conn, err := grpcClient(config)
		if err != nil {
			// Close any already created connections on error
			for _, existingConn := range connections {
				existingConn.Close()
			}
			return nil, fmt.Errorf("failed to create gRPC connection to %s: %w", config.Addr, err)
		}
		connections = append(connections, conn)
	}
	
	return &MultiGRPCClient{
		connections: connections,
		configs:     configs,
	}, nil
}

// GetConnection returns the first available connection
func (m *MultiGRPCClient) GetConnection() *grpc.ClientConn {
	if len(m.connections) > 0 {
		return m.connections[0]
	}
	return nil
}

// GetAllConnections returns all connections for advanced usage
func (m *MultiGRPCClient) GetAllConnections() []*grpc.ClientConn {
	return m.connections
}

// Close closes all gRPC connections
func (m *MultiGRPCClient) Close() error {
	var errs []error
	for i, conn := range m.connections {
		if err := conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close connection %d (%s): %w", i, m.configs[i].Addr, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// CoreGRPCConfig is the configuration for the core gRPC client.
type CoreGRPCConfig struct {
	// Addr is the address of the core gRPC server.
	Addr string
	// AdditionalCoreGRPCConfigs is a list of additional core gRPC server configurations for failover
	AdditionalCoreGRPCConfigs []CoreGRPCConfig
	// TLSEnabled specifies whether the connection is secure or not.
	TLSEnabled bool
	// AuthToken is the authentication token to be used for gRPC authentication.
	// If left empty, the client will not include the authentication token in its requests.
	// Note: AuthToken is insecure without TLS
	AuthToken string
}

// Validate performs basic validation of the config.
func (cfg *CoreGRPCConfig) Validate() error {
	_, err := utils.SanitizeAddr(cfg.Addr)
	if err != nil {
		return err
	}
	
	// Validate additional core gRPC configurations
	for i, additionalCfg := range cfg.AdditionalCoreGRPCConfigs {
		if err := additionalCfg.Validate(); err != nil {
			return fmt.Errorf("invalid additional core gRPC config at index %d: %w", i, err)
		}
	}
	
	return nil
}

func grpcClient(cfg CoreGRPCConfig) (*grpc.ClientConn, error) {
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
