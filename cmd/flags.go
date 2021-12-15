package cmd

import (
	"github.com/spf13/pflag"
)

var configFlags = []*pflag.Flag{
	loglevelFlag,
	nodeConfigFlag,
	trustedHashFlag,
	trustedPeerFlag,
	coreRemoteFlag,
}

var (
	loglevelFlag = &pflag.Flag{
		Name:     "log.level",
		Usage:    "DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL, and\n// their lower-case forms",
		DefValue: "info",
	}
	nodeConfigFlag = &pflag.Flag{
		Name:     "node.config",
		Usage:    "Path to a customized Config",
		DefValue: "",
	}
	trustedHashFlag = &pflag.Flag{
		Name:     "headers.trusted-hash",
		Usage:    "Hex encoded block hash. Starting point for header synchronization",
		DefValue: "",
	}
	trustedPeerFlag = &pflag.Flag{
		Name:     "headers.trusted-peer",
		Usage:    "Multiaddr of a reliable peer to fetch headers from",
		DefValue: "",
	}
	coreRemoteFlag = &pflag.Flag{
		Name: "core.remote",
		Usage: "Indicates node to connect to the given remote core node. " +
			"Example: <protocol>://<ip>:<port>, tcp://127.0.0.1:26657",
		DefValue: "",
	}
)
