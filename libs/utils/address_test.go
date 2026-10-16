package utils

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

const ipv6Loopback = "::1"

func TestSanitizeAddr(t *testing.T) {
	tests := []struct {
		addr string
		want string
		err  error
	}{
		// Testcase: trims protocol prefix
		{addr: "http://celestia.org", want: "celestia.org"},
		// Testcase: protocol prefix trimmed already
		{addr: "celestia.org", want: "celestia.org"},
		// Testcase: trims protocol prefix, and trims port and trailing slash suffix
		{addr: "tcp://192.168.42.42:5050/", want: "192.168.42.42"},
		// Testcase: invariant ip
		{addr: "192.168.42.42", want: "192.168.42.42"},
		// Testcase: invariant IPv6
		{addr: ipv6Loopback, want: ipv6Loopback},
		// Testcase: trims IPv6 brackets
		{addr: "[::1]", want: ipv6Loopback},
		// Testcase: trims IPv6 brackets and port
		{addr: "[::1]:5050", want: ipv6Loopback},
		// Testcase: trims protocol prefix, IPv6 brackets, port, and trailing slash suffix
		{addr: "https://[2001:db8::1]:5050/", want: "2001:db8::1"},
		// Testcase: empty addr
		{addr: "", want: "", err: ErrInvalidIP},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got, err := SanitizeAddr(tt.addr)
			require.Equal(t, tt.want, got)
			require.ErrorIs(t, err, tt.err)
		})
	}
}

func TestValidateAddr(t *testing.T) {
	type want struct {
		addr       string
		unresolved bool
	}
	tests := []struct {
		addr string
		want want
	}{
		// Testcase: ip is valid
		{addr: "192.168.42.42:5050", want: want{addr: "192.168.42.42"}},
		// Testcase: ip is valid, no port
		{addr: "192.168.42.42", want: want{addr: "192.168.42.42"}},
		// Testcase: IPv6 is valid, no port
		{addr: ipv6Loopback, want: want{addr: ipv6Loopback}},
		// Testcase: IPv6 is valid with brackets and port
		{addr: "[2001:db8::1]:5050", want: want{addr: "2001:db8::1"}},
		// Testcase: resolves localhost
		{addr: "http://localhost:8080/", want: want{unresolved: true}},
		// Testcase: hostname is valid
		{addr: "https://celestia.org", want: want{unresolved: true}},
		// Testcase: hostname is valid, but no schema
		{addr: "celestia.org", want: want{unresolved: true}},
		// Testcase: localhost
		{addr: "localhost", want: want{addr: "127.0.0.1"}},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got, err := ValidateAddr(tt.addr)
			require.NoError(t, err)

			// validate that returned value is ip
			if ip := net.ParseIP(got); ip == nil {
				t.Fatalf("empty ip")
			}

			if tt.want.unresolved {
				// unresolved addr has no addr to compare with
				return
			}
			require.Equal(t, tt.want.addr, got)
		})
	}
}

func TestNormalizeGRPCAddress(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{addr: "https://your-quicknode-url.celestia-mocha.quiknode.pro:9090", want: "your-quicknode-url.celestia-mocha.quiknode.pro:9090"},
		{addr: "http://localhost:9090", want: "localhost:9090"},
		{addr: "https://host.example.com:9090/some/path", want: "host.example.com:9090"},
		{addr: "host.example.com:9090/some/path", want: "host.example.com:9090"},
		{addr: "localhost:9090", want: "localhost:9090"},
		{addr: "localhost", want: "localhost"},
		{addr: "https://localhost", want: "localhost"},
		{addr: "tcp://localhost:9090", want: "localhost:9090"},
		{addr: "[::1]:9090", want: "[::1]:9090"},
		{addr: "https://[::1]:9090", want: "[::1]:9090"},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got := NormalizeGRPCAddress(tt.addr)
			require.Equal(t, tt.want, got)
		})
	}
}
