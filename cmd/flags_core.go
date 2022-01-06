package cmd

import (
	"fmt"
	"net"
	"net/url"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/node"
)

var (
	coreRemoteFlag = "core.remote"
)

func CoreFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		coreRemoteFlag,
		"",
		"Indicates node to connect to the given remote core node. "+
			"Example: <protocol>://<ip>:<port>, tcp://127.0.0.1:26657",
	)

	return flags
}

func ParseCoreFlags(cmd *cobra.Command, env *Env) error {
	coreRemote := cmd.Flag(coreRemoteFlag).Value.String()
	if coreRemote != "" {
		proto, addr, err := validateAddress(coreRemote)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s': %w", coreRemoteFlag, err)
		}

		env.addOption(node.WithRemoteCore(proto, addr))
	}

	return nil
}

// validateAddress parses the given address of the remote core node
// and checks if it configures correctly
func validateAddress(address string) (string, string, error) {
	u, err := url.Parse(address)
	if err != nil {
		return "", "", err
	}

	if u.Scheme == "" || u.Host == "" {
		return "", "", fmt.Errorf("both protocol and host must present in the address")
	}

	if _, port, err := net.SplitHostPort(u.Host); err != nil || port == "" {
		return "", "", fmt.Errorf("incorrect address provided for Remote Core")
	}

	return u.Scheme, u.Host, nil
}
