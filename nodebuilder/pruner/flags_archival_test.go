package pruner

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func parsedConfig(t *testing.T, tp node.Type, args ...string) *Config {
	cmd := &cobra.Command{Use: "start", Run: func(*cobra.Command, []string) {}}
	cmd.Flags().AddFlagSet(Flags())
	require.NoError(t, cmd.ParseFlags(args))

	opt := ParseFlags(cmd, tp)
	if opt == nil {
		opt = fx.Options()
	}
	var cfg *Config
	app := fxtest.New(t, fx.Supply(DefaultConfig()), opt, fx.Populate(&cfg))
	app.RequireStart().RequireStop()
	return cfg
}

// TestParseFlags_ArchivalFalse verifies that `--archival=false` keeps the default
// pruned mode while `--archival` enables archival mode.
func TestParseFlags_ArchivalFalse(t *testing.T) {
	require.True(t, parsedConfig(t, node.Bridge).EnableService, "default must be pruned mode")
	require.False(t, parsedConfig(t, node.Bridge, "--archival").EnableService,
		"--archival must enable archival mode")
	require.True(t, parsedConfig(t, node.Bridge, "--archival=false").EnableService,
		"--archival=false switched the node into archival mode")

	// archival mode is Bridge-only, but an explicit false is fine on other node types
	require.True(t, parsedConfig(t, node.Light, "--archival=false").EnableService)
}
