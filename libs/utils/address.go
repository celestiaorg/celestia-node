package utils

import (
	"fmt"
	"net"
	"strings"
)

// SanitizeAddr trims leading protocol scheme and port from the given
// IP address or hostname if present.
func SanitizeAddr(addr string) (string, error) {
	original := addr
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	addr = strings.TrimPrefix(addr, "tcp://")
	addr = strings.TrimSuffix(addr, "/")
	addr = strings.Split(addr, ":")[0]
	if addr == "" {
		return "", fmt.Errorf("invalid IP address or hostname given: %s", original)
	}
	return addr, nil
}

// ValidateAddr sanitizes the given address and verifies that it is a valid IP or hostname. The
// sanitized address is returned.
func ValidateAddr(addr string) (string, error) {
	addr, err := SanitizeAddr(addr)
	if err != nil {
		return addr, err
	}

	if ip := net.ParseIP(addr); ip == nil {
		_, err = net.LookupHost(addr)
		if err != nil {
			return addr, fmt.Errorf("could not resolve hostname or ip: %w", err)
		}
	}

	return addr, nil
}
