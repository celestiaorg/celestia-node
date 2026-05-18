package das

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// TestConstructModule_DASDisabledStub verifies that when DAS is disabled, it implements a stub
// daser that returns an error and empty das.SamplingStats
func TestConstructModule_DASDisabledStub(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var mod Module

	cfg := DefaultConfig(node.Light)
	cfg.Enabled = false // Explicitly disable DAS
	app := fxtest.New(t,
		ConstructModule(&cfg),
		fx.Populate(&mod)).
		RequireStart()
	defer app.RequireStop()

	_, err := mod.SamplingStats(ctx)
	assert.ErrorIs(t, err, errStub)
}
