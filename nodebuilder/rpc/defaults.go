package rpc

const (
	defaultBindAddress = "localhost"
	defaultPort        = "26658"
)

var (
	defaultAllowedMethods = []string{"GET", "POST", "OPTIONS"}
	defaultAllowedHeaders = []string{"Content-Type", "Authorization"}
)
