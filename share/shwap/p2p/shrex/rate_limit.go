package shrex

import (
	"net/netip"
	"os"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	libp2prate "github.com/libp2p/go-libp2p/x/rate"
	manet "github.com/multiformats/go-multiaddr/net"
)

// disableRateLimiting disables the per-IP rate limiter when set to "1".
// Intended for debugging and testing; do not use in production.
var disableRateLimiting = os.Getenv("CELESTIA_SHREX_DISABLE_RATE_LIMITING") == "1"

// rateLimitPerPeer is the sustained request rate allowed per peer IP (requests/sec).
// A light node runs 16 workers × 16 samples = 256 requests per DAS round. With
// a ~3s block time that is ~85 req/s in steady state, allowing the bucket to
// refill one full DAS round worth of tokens between blocks.
//
// Rate limiting is applied globally per IP across all shrex protocol types. Expensive
// requests (EdsID, RowID) are naturally bounded by their per-request memory cost and
// disk I/O — the rcmgr memory budget and physical disk limits constrain them more
// tightly than any token bucket could. The rate limiter exists primarily to prevent
// sequential flooding of cheap requests (SampleID) that can cycle rapidly under the
// rcmgr concurrency cap.
var rateLimitPerPeer = 85.0

// rateBurstPerPeer is the token-bucket burst size per peer IP.
// Sized for one full DAS round from a single light node: 16 workers × 16 samples
// = 256 streams opened nearly simultaneously.
var rateBurstPerPeer = 256

// newPeerRateLimiter returns a per-IP rate limiter for the shrex server.
// Returns nil if rate limiting is disabled via CELESTIA_SHREX_DISABLE_RATE_LIMITING.
//
// Limiting is per individual IP (/32 IPv4, /128 IPv6) for Sybil resistance: a peer
// cycling multiple identities from the same IP shares one bucket. The trade-off is
// that nodes behind shared NAT compete for the same bucket; this is accepted since
// Celestia node operators typically run on dedicated IPs.
//
// Loopback addresses (127.0.0.0/8, ::1/128) are exempted so localhost connections
// (tests, local tooling) are never rejected. libp2prate treats Limit{} (RPS==0) as
// unlimited (rate.Inf), matching the convention used by libp2p's identify protocol.
//
// The libp2p x/rate package handles bucket memory cleanup automatically via expiry.
func newPeerRateLimiter() *libp2prate.Limiter {
	if disableRateLimiting {
		log.Warn("server: rate limiting disabled via CELESTIA_SHREX_DISABLE_RATE_LIMITING")
		return nil
	}
	limit := libp2prate.Limit{RPS: rateLimitPerPeer, Burst: rateBurstPerPeer}
	return &libp2prate.Limiter{
		NetworkPrefixLimits: []libp2prate.PrefixLimit{
			{Prefix: netip.MustParsePrefix("127.0.0.0/8"), Limit: libp2prate.Limit{}},
			{Prefix: netip.MustParsePrefix("::1/128"), Limit: libp2prate.Limit{}},
		},
		SubnetRateLimiter: libp2prate.SubnetLimiter{
			IPv4SubnetLimits: []libp2prate.SubnetLimit{{PrefixLength: 32, Limit: limit}},
			IPv6SubnetLimits: []libp2prate.SubnetLimit{{PrefixLength: 128, Limit: limit}},
			GracePeriod:      time.Minute,
		},
	}
}

// remoteIP extracts the IP address of the remote peer from a stream's connection multiaddr.
// Returns the zero (invalid) netip.Addr when the address cannot be parsed, e.g. for relay
// or circuit-relay transports. An invalid addr causes SubnetLimiter.Allow to return false
// because netip.Addr{}.Prefix() errors, which the limiter treats as denied. Non-IP
// connections are therefore blocked outright, which is acceptable: Celestia peers
// communicate exclusively over IP.
func remoteIP(s network.Stream) netip.Addr {
	ip, err := manet.ToIP(s.Conn().RemoteMultiaddr())
	if err != nil {
		return netip.Addr{}
	}
	addr, _ := netip.AddrFromSlice(ip)
	return addr.Unmap()
}
