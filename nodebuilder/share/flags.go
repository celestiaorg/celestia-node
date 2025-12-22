package share

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

const (
	namespaceIDFlag = "share.namespace_id"
)

// Flags gives a set of share flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		namespaceIDFlag,
		"",
		"The specific Namespace ID to store. Only shares with this ID will be kept on disk.",
	)

	return flags
}

// ParseFlags parses share flags from the given cmd and saves them to the passed config.
func ParseFlags(
	cmd *cobra.Command,
	cfg *Config,
) error {
	namespaceID, err := cmd.Flags().GetString(namespaceIDFlag)
	if err != nil {
		return err
	}
	if namespaceID != "" {
		cfg.NamespaceID = namespaceID
	}
	return nil
}
