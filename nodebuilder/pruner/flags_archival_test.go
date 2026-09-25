package pruner

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func parsedConfig(t *testing.T, args ...string) *Config {
	cmd := &cobra.Command{Use: "start", Run: func(*cobra.Command, []string) {}}
	cmd.Flags().AddFlagSet(Flags())
	require.NoError(t, cmd.ParseFlags(args))

	opt := ParseFlags(cmd, node.Bridge)
	if opt == nil {
		opt = fx.Options()
	}
	var cfg *Config
	app := fxtest.New(t, fx.Supply(DefaultConfig()), opt, fx.Populate(&cfg))
	app.RequireStart().RequireStop()
	return cfg
}

// TestParseFlags_ArchivalFalse: `--archival=false` must keep the default pruned mode,
// but ParseFlags only checks Flag.Changed and ignores the parsed value.
func TestParseFlags_ArchivalFalse(t *testing.T) {
	require.True(t, parsedConfig(t).EnableService, "default must be pruned mode")
	require.False(t, parsedConfig(t, "--archival").EnableService, "--archival must enable archival mode")
	require.True(t, parsedConfig(t, "--archival=false").EnableService,
		"--archival=false switched the node into archival mode")
}
