package telemetry

import "go.uber.org/fx"

const instrumentationName = "celestia-node"

func Module() fx.Option {
	return fx.Options(
		tracing(),
		metrics(),
	)
}
