package core

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/fx/fxtest"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-node/internal/grpctest"
)

func TestGRPCClientAuthUsesSingleRetryInterceptor(t *testing.T) {
	grpctest.VerifySingleRetryInterceptor(t, func(
		t *testing.T,
		addr string,
		token string,
	) (*grpc.ClientConn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		tokenDir := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(tokenDir, "xtoken.json"),
			[]byte(`{"x-token":"`+token+`"}`),
			0o600,
		); err != nil {
			return nil, err
		}

		return grpcClient(fxtest.NewLifecycle(t), EndpointConfig{
			IP:         host,
			Port:       port,
			XTokenPath: tokenDir,
		})
	})
}
