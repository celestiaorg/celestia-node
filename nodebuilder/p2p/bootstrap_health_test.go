//go:build bootstrapper_health

package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/stretchr/testify/require"
)

func TestBootstrapperHealth(t *testing.T) {
	networks := []Network{Mainnet, Mocha, Arabica}
	for _, network := range networks {
		t.Run(string(network), func(t *testing.T) {
			bootstrappers, err := BootstrappersFor(network)
			require.NoError(t, err)
			require.NotEmpty(t, bootstrappers, "no bootstrappers found for network %s", network)

			for _, bootstrapper := range bootstrappers {
				t.Run(bootstrapper.ID.String(), func(t *testing.T) {
					t.Parallel()

					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()

					h, err := libp2p.New(libp2p.NoListenAddrs)
					require.NoError(t, err)
					defer h.Close()

					err = h.Connect(ctx, bootstrapper)
					require.NoError(t, err, "failed to connect to bootstrapper %s on network %s", bootstrapper, network)
				})
			}
		})
	}
}
