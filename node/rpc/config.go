package rpc

// Config represents the configuration for the RPC client
// used for retrieving information from Celestia Core nodes // TODO @renaynay: update this post-devnet
type Config struct {
	Protocol   string
	RemoteAddr string
}
