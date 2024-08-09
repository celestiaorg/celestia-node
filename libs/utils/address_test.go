package utils

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

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
