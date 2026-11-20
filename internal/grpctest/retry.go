package grpctest

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type retryHealthServer struct {
	healthpb.UnimplementedHealthServer
	token          string
	unaryAttempts  atomic.Int32
	streamAttempts atomic.Int32
}

func (s *retryHealthServer) Check(
	ctx context.Context,
	_ *healthpb.HealthCheckRequest,
) (*healthpb.HealthCheckResponse, error) {
	if !hasToken(ctx, s.token) {
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}
	if s.unaryAttempts.Add(1) == 1 {
		return nil, status.Error(codes.Unavailable, "retry")
	}
	return &healthpb.HealthCheckResponse{}, nil
}

func (s *retryHealthServer) Watch(
	_ *healthpb.HealthCheckRequest,
	stream grpc.ServerStreamingServer[healthpb.HealthCheckResponse],
) error {
	if !hasToken(stream.Context(), s.token) {
		return status.Error(codes.Unauthenticated, "missing token")
	}
	if s.streamAttempts.Add(1) == 1 {
		return status.Error(codes.Unavailable, "retry")
	}
	return stream.Send(&healthpb.HealthCheckResponse{})
}

// VerifySingleRetryInterceptor checks that per-call options reach the client's retry interceptor.
func VerifySingleRetryInterceptor(
	t *testing.T,
	newClient func(*testing.T, string, string) (*grpc.ClientConn, error),
) {
	t.Helper()
	const token = "test-token"

	t.Run("unary", func(t *testing.T) {
		addr, server := startServer(t, token)
		conn := connect(t, newClient, addr, token)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := healthpb.NewHealthClient(conn).Check(
			ctx,
			&healthpb.HealthCheckRequest{},
			grpc_retry.Disable(),
		)
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Equal(t, int32(1), server.unaryAttempts.Load())
	})

	t.Run("stream", func(t *testing.T) {
		addr, server := startServer(t, token)
		conn := connect(t, newClient, addr, token)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		stream, err := healthpb.NewHealthClient(conn).Watch(
			ctx,
			&healthpb.HealthCheckRequest{},
			grpc_retry.Disable(),
		)
		require.NoError(t, err)
		_, err = stream.Recv()
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.Equal(t, int32(1), server.streamAttempts.Load())
	})
}

func connect(
	t *testing.T,
	newClient func(*testing.T, string, string) (*grpc.ClientConn, error),
	addr string,
	token string,
) *grpc.ClientConn {
	t.Helper()
	conn, err := newClient(t, addr, token)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return conn
}

func startServer(t *testing.T, token string) (string, *retryHealthServer) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &retryHealthServer{token: token}
	grpcServer := grpc.NewServer()
	healthpb.RegisterHealthServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)
	return listener.Addr().String(), server
}

func hasToken(ctx context.Context, token string) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	tokens := md.Get("x-token")
	return ok && len(tokens) == 1 && tokens[0] == token
}
