package das

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// TestConstructModule_DASBridgeStub verifies that a bridge node implements a stub daser that
// returns an error and empty das.SamplingStats
func TestConstructModule_DASBridgeStub(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var mod Module

	cfg := DefaultConfig(node.Bridge)
	app := fxtest.New(t,
		ConstructModule(node.Bridge, &cfg),
		fx.Populate(&mod)).
		RequireStart()
	defer app.RequireStop()

	_, err := mod.SamplingStats(ctx)
	assert.ErrorIs(t, err, errStub)
}
