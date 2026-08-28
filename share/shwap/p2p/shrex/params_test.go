package shrex

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"
)

func TestProtocolIDs(t *testing.T) {
	require.Equal(t, []protocol.ID{
		"/test/shrex/v0.1.0/nd_v0",
		"/test/shrex/v0.1.0/eds_v0",
		"/test/shrex/v0.1.0/sample_v0",
		"/test/shrex/v0.1.0/row_v0",
		"/test/shrex/v0.1.0/rangeNamespaceData_v0",
	}, ProtocolIDs("test"))
}
