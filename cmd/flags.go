package cmd

import "flag"

var configFlags = []*flag.Flag{
	loglevelFlag,
	nodeConfigFlag,
	trustedHashFlag,
	trustedPeerFlag,
	coreRemoteFlag,
	mutualPeersFlag,
}

var (
	loglevelFlag = &flag.Flag{
		Name:     "log.level",
		Usage:    "DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL, and\n// their lower-case forms",
		DefValue: "info",
	}
	nodeConfigFlag = &flag.Flag{
		Name:     "node.config",
		Usage:    "Path to a customized Config",
		DefValue: "",
	}
	trustedHashFlag = &flag.Flag{
		Name:     "headers.trusted-hash",
		Usage:    "Hex encoded block hash. Starting point for header synchronization",
		DefValue: "",
	}
	trustedPeerFlag = &flag.Flag{
		Name:     "headers.trusted-peer",
		Usage:    "Multiaddr of a reliable peer to fetch headers from",
		DefValue: "",
	}
	coreRemoteFlag = &flag.Flag{
		Name: "core.remote",
		Usage: "Indicates node to connect to the given remote core node. " +
			"Example: <protocol>://<ip>:<port>, tcp://127.0.0.1:26657",
		DefValue: "",
	}
	mutualPeersFlag = &flag.Flag{
		Name: "p2p.mutual",
		Usage: "Comma-separated Multiaddresses of mutual peers to keep unbreakable connection with. " +
			"Peers must bidirectionally point to each other.",
		DefValue: "",
	}
)
