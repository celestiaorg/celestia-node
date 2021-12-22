package cmd

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/logs"
)

var (
	logLevelF = "log.level"
)

func MiscFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		logLevelF,
		"INFO",
		`DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL
and their lower-case forms`,
	)

	return flags
}

func ParseMiscFlags(cmd *cobra.Command) error {
	logLevel := cmd.Flag(logLevelF).Value.String()
	if logLevel != "" {
		level, err := logging.LevelFromString(logLevel)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s': %w", logLevelF, err)
		}

		logs.SetAllLoggers(level)
	}

	return nil
}
