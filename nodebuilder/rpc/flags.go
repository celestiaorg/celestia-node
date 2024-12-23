package rpc

import (
	"fmt"
	"net"
	"strconv"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	log      = logging.Logger("rpc")
	addrFlag = "rpc.addr"
	portFlag = "rpc.port"
	authFlag = "rpc.skip-auth"
)

// Flags gives a set of hardcoded node/rpc package flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		addrFlag,
		"",
		fmt.Sprintf("Set a custom RPC listen address (default: %s)", defaultBindAddress),
	)
	flags.String(
		portFlag,
		"",
		fmt.Sprintf("Set a custom RPC port (default: %s)", defaultPort),
	)
	flags.Bool(
		authFlag,
		false,
		"Skips authentication for RPC requests",
	)

	return flags
}

// ParseFlags parses RPC flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) error {
	cfg.Address = cmd.Flag(addrFlag).Value.String()
	cfg.Port = cmd.Flag(portFlag).Value.String()

	// If address and port are not specified, use default values
	if cfg.Address == "" {
		cfg.Address = defaultBindAddress
	}
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}

	return cfg.Validate()
}

func (c *Config) Validate() error {
	if c.Address != "" {
		host, port, err := net.SplitHostPort(c.Address)
		if err != nil {
			return fmt.Errorf("invalid address format: %v", err)
		}
		if port != "" {
			portNum, err := strconv.Atoi(port)
			if err != nil || portNum < 1 || portNum > 65535 {
				return fmt.Errorf("invalid port number")
			}
		}
		if host != "" {
			if ip := net.ParseIP(host); ip == nil {
				return fmt.Errorf("invalid IP address")
			}
		}
	}
	if c.Port != "" {
		portNum, err := strconv.Atoi(c.Port)
		if err != nil || portNum < 1 || portNum > 65535 {
			return fmt.Errorf("invalid port number")
		}
	}
	return nil
}
