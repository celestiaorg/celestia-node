package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeAddr(t *testing.T) {
	var tests = []struct {
		addr string
		want string
	}{
		// Testcase: trims protocol prefix
		{addr: "http://celestia.org", want: "celestia.org"},
		// Testcase: trims protocol prefix, and trims port and trailing slash suffix
		{addr: "tcp://192.168.42.42:5050/", want: "192.168.42.42"},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got, err := SanitizeAddr(tt.addr)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestValidateAddr(t *testing.T) {
	var tests = []struct {
		addr string
		want string
	}{
		// Testcase: ip is valid
		{addr: "192.168.42.42:5050", want: "192.168.42.42"},
		// Testcase: hostname is valid
		{addr: "https://celestia.org", want: "celestia.org"},
		// Testcase: resolves localhost
		{addr: "http://localhost:8080/", want: "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got, err := ValidateAddr(tt.addr)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
