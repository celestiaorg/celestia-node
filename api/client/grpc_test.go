package client

import (
	"testing"

	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-node/internal/grpctest"
)

func TestGRPCClientAuthUsesSingleRetryInterceptor(t *testing.T) {
	grpctest.VerifySingleRetryInterceptor(t, func(
		_ *testing.T,
		addr string,
		token string,
	) (*grpc.ClientConn, error) {
		return grpcClient(CoreGRPCConfig{Addr: addr, AuthToken: token})
	})
}
