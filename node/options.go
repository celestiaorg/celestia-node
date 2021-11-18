package node

import tmbytes "github.com/tendermint/tendermint/libs/bytes"

// Options defines how to setup config
type Options func(*Config)

// WithRemoteClient configures node to start on remote address
func WithRemoteClient(protocol string, address string) Options {
	return func(args *Config) {
		args.Core.Remote = true
		args.Core.RemoteConfig.Protocol = protocol
		args.Core.RemoteConfig.RemoteAddr = address
	}
}

func WithHead(hash tmbytes.HexBytes) Options {
	return func(args *Config) {
		args.HeadHash = hash
	}
}
