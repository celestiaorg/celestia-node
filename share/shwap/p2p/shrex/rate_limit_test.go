package shrex

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPeerRateLimiter_IPv6SharesSubnetBucket(t *testing.T) {
	lim := newPeerRateLimiter()
	require.NotNil(t, lim)

	subnet := netip.MustParsePrefix("2001:db8:1:2::/64").Addr().As16()
	allowed := 0
	for i := range 4 * rateBurstPerPeer {
		addr := subnet
		binary.BigEndian.PutUint64(addr[8:], uint64(i+1)) // different address in the same /64
		if lim.Allow(netip.AddrFrom16(addr)) {
			allowed++
		}
	}
	// requests are back to back, so only the burst (plus a few refilled tokens) is allowed
	require.Less(t, allowed, 2*rateBurstPerPeer)

	other := netip.MustParseAddr("2001:db8:1:3::1")
	require.True(t, lim.Allow(other), "another /64 must have its own bucket")
}
