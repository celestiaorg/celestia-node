// Package rpc provides RPC configuration defaults
//
//nolint:revive // rpc is a common package name in this codebase
package rpc

const (
	defaultBindAddress = "localhost"
	defaultPort        = "26658"
)

var (
	defaultAllowedMethods = []string{"GET", "POST", "OPTIONS"}
	defaultAllowedHeaders = []string{"Content-Type", "Authorization"}
)
