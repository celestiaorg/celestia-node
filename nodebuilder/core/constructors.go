package core

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

const xtokenFileName = "xtoken.json"

func grpcClient(lc fx.Lifecycle, cfg Config) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if cfg.TLSEnabled {
		opts = append(opts, grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})),
		)
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if cfg.XTokenPath != "" {
		xToken, err := parseTokenPath(cfg.XTokenPath)
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithUnaryInterceptor(authInterceptor(xToken)))
		opts = append(opts, grpc.WithStreamInterceptor(authStreamInterceptor(xToken)))
	}

	endpoint := net.JoinHostPort(cfg.IP, cfg.Port)
	conn, err := NewGRPCClient(endpoint, opts...)
	if err != nil {
		return nil, err
	}
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
