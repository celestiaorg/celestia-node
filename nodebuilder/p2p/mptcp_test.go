package p2p

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMPTCPListenersDisabledByDefault(t *testing.T) {
	t.Setenv("GODEBUG", "")
	var lc net.ListenConfig
	require.False(t, lc.MultipathTCP())
}
