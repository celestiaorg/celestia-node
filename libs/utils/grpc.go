package utils

import (
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// gRPC client requires fetching a block on initialization that can be larger
// than the default message size set in gRPC. Increasing defaults up to 64MB
// to avoid fixing it every time the block size increases.
// Tested on mainnet node:
// square size = 128
// actual response size = 10,85mb
// TODO(@vgonkivs): Revisit this constant once the block size reaches 64MB.
var defaultGRPCMessageSize = 64 * 1024 * 1024 // 64Mb

func ConstructGRPCConnection(addr string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts = append(opts, grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(defaultGRPCMessageSize),
		grpc.MaxCallSendMsgSize(defaultGRPCMessageSize),
	))

	return grpc.NewClient(addr, opts...)
}

func GRPCRetryInterceptor() grpc.UnaryClientInterceptor {
	return grpc_retry.UnaryClientInterceptor(
		grpc_retry.WithMax(5),
		grpc_retry.WithCodes(codes.Unavailable),
		grpc_retry.WithBackoff(
			grpc_retry.BackoffExponentialWithJitter(time.Second, 2.0)),
	)
}
