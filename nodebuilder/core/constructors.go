package core

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

const (
	// gRPC client requires fetching a block on initialization that can be larger
	// than the default message size set in gRPC. Increasing defaults up to 64MB
	// to avoid fixing it every time the block size increases.
	// Tested on mainnet node:
	// square size = 128
	// actual response size = 10,85mb
	// TODO(@vgonkivs): Revisit this constant once the block size reaches 64MB.
	defaultGRPCMessageSize = 64 * 1024 * 1024 // 64Mb
	xtokenFileName         = "xtoken.json"
)

// singleGRPCClient creates a single gRPC client connection for a given endpoint
func singleGRPCClient(lc fx.Lifecycle, endpoint Endpoint) (*grpc.ClientConn, error) {
	// Base options that apply to all connections
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(defaultGRPCMessageSize),
			grpc.MaxCallSendMsgSize(defaultGRPCMessageSize),
		),
	}

	// Configure TLS or insecure connection based on the endpoint configuration
	if endpoint.TLSEnabled {
		opts = append(opts, grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})),
		)
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Configure retry interceptors
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

	// Add authentication token if configured
	if endpoint.XTokenPath != "" {
		xToken, err := parseTokenPath(endpoint.XTokenPath)
		if err != nil {
			return nil, err
		}
		opts = append(opts,
			grpc.WithChainUnaryInterceptor(authInterceptor(xToken), retryInterceptor),
			grpc.WithChainStreamInterceptor(authStreamInterceptor(xToken), retryStreamInterceptor),
		)
	}

	addr := net.JoinHostPort(endpoint.IP, endpoint.Port)
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, err
	}

	// Add lifecycle hooks for proper connection management
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			conn.Connect()
			if !conn.WaitForStateChange(ctx, connectivity.Ready) {
				return errors.New("couldn't connect to core endpoint")
			}
			return nil
		},
		OnStop: func(context.Context) error {
			return conn.Close()
		},
	})
	return conn, nil
}

// grpcClients creates gRPC client connections for all configured endpoints.
// The connections are used by the BlockFetcher for load balancing in a round-robin fashion.
func grpcClients(lc fx.Lifecycle, cfg Config) ([]*grpc.ClientConn, error) {
	endpoints := cfg.GetAllEndpoints()

	if len(endpoints) == 0 {
		return nil, errors.New("no core endpoints configured")
	}

	conns := make([]*grpc.ClientConn, 0, len(endpoints))

	// Create a connection for each endpoint
	for _, endpoint := range endpoints {
		conn, err := singleGRPCClient(lc, endpoint)
		if err != nil {
			// Clean up all previously created connections on error
			for _, c := range conns {
				c.Close()
			}
			return nil, err
		}
		conns = append(conns, conn)
	}

	return conns, nil
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

// parseTokenPath retrieves the authentication token from a JSON file at the specified path.
func parseTokenPath(xtokenPath string) (string, error) {
	xtokenPath = filepath.Join(xtokenPath, xtokenFileName)
	exist := utils.Exists(xtokenPath)
	if !exist {
		return "", os.ErrNotExist
	}

	token, err := os.ReadFile(xtokenPath)
	if err != nil {
		return "", err
	}

	auth := struct {
		Token string `json:"x-token"`
	}{}

	err = json.Unmarshal(token, &auth)
	if err != nil {
		return "", err
	}
	if auth.Token == "" {
		return "", errors.New("x-token is empty. Please setup a token or cleanup xtokenPath")
	}
	return auth.Token, nil
}
