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

	xtokenFileName = "xtoken.json"
)

type AdditionalCoreConns []*grpc.ClientConn

// TODO @renaynay: should we make this reusable so we can have all auth + other features
// for the estimator service too?
func grpcClient(lc fx.Lifecycle, cfg EndpointConfig) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
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

	if cfg.XTokenPath != "" {
		xToken, err := parseTokenPath(cfg.XTokenPath)
		if err != nil {
			return nil, err
		}
		opts = append(opts,
			grpc.WithChainUnaryInterceptor(authInterceptor(xToken), retryInterceptor),
			grpc.WithChainStreamInterceptor(authStreamInterceptor(xToken), retryStreamInterceptor),
		)
	}
	opts = append(opts, grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(defaultGRPCMessageSize),
		grpc.MaxCallSendMsgSize(defaultGRPCMessageSize),
	))

	conn, err := grpc.NewClient(net.JoinHostPort(cfg.IP, cfg.Port), opts...)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			conn.Connect()
			for {
				state := conn.GetState()
				if state == connectivity.Ready {
					break
				}
				if state == connectivity.Shutdown {
					return errors.New("couldn't connect to core endpoint")
				}
				if !conn.WaitForStateChange(ctx, state) {
					return errors.New("couldn't connect to core endpoint")
				}
			}
			return nil
		},
		OnStop: func(context.Context) error {
			return conn.Close()
		},
	})
	return conn, nil
}

func additionalCoreEndpointGrpcClients(lc fx.Lifecycle, cfg Config) (AdditionalCoreConns, error) {
	additionalEndpoints := make(AdditionalCoreConns, 0, len(cfg.AdditionalCoreEndpoints))
	for _, additionalCfg := range cfg.AdditionalCoreEndpoints {
		endpoint, err := grpcClient(lc, additionalCfg)
		if err != nil {
			return nil, err
		}
		additionalEndpoints = append(additionalEndpoints, endpoint)
	}

	return additionalEndpoints, nil
}

func authInterceptor(xtoken string) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
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
