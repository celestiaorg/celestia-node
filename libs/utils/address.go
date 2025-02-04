package utils

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
)

var ErrInvalidIP = errors.New("invalid IP address or hostname given")

// NormalizeAddress extracts the host and port, removing unsupported schemes.
func NormalizeAddress(addr string) string {
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimPrefix(addr, "tcp://")
	addr = strings.TrimSuffix(addr, "/")
	return addr
}

// SanitizeAddr trims leading protocol scheme and port from the given
// IP address or hostname if present.
func SanitizeAddr(addr string) (string, error) {
	original := addr
	addr = NormalizeAddress(addr)
	addr = strings.Split(addr, ":")[0]
	if addr == "" {
		return "", fmt.Errorf("%w: %s", ErrInvalidIP, original)
	}
	return addr, nil
}

// ValidateAddr sanitizes the given address and verifies that it
// is a valid IP or hostname. The sanitized address is returned.
func ValidateAddr(addr string) (string, error) {
	addr, err := SanitizeAddr(addr)
	if err != nil {
		return addr, err
	}

	// if the address is a valid IP, return it
	ip, err := netip.ParseAddr(addr)
	if err == nil {
		return ip.String(), nil
	}

	// if the address is not a valid IP, resolve it
	resolved, err := net.ResolveIPAddr("ip4", addr)
	if err != nil {
		return addr, err
	}
	return resolved.String(), nil
}
