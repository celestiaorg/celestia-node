package canary

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

func TestSummaryRendersVerdictChecksAndMetrics(t *testing.T) {
	seconds, connected, total, rate := 2.3, 5, 6, 239.4
	r := model.Result{
		Profile: model.Profile{
			Name:         model.ProfileMainnet,
			Release:      "v0.33.0",
			Image:        "ghcr.io/celestiaorg/celestia-node@sha256:abc",
			SourceCommit: "9e954e900951ba06c0e7b8502a7361ffdbd6f91f",
		},
		Outcome: model.Pass,
		Checks: []model.Check{
			{Name: model.CheckDAS, Outcome: model.Pass, Code: "native_sample_observed", DurationSeconds: 0.1},
			{Name: model.CheckDASRecent, Outcome: model.Inconclusive, Code: "recent_sampling_timeout", DurationSeconds: 180},
		},
		Metrics: model.Metrics{
			StartupSeconds:         &seconds,
			BootstrappersConnected: &connected,
			BootstrappersTotal:     &total,
			SyncHeadersPerSecond:   &rate,
		},
	}
	s := Summary(r)
	require.Contains(t, s, "### Fresh node canary: mainnet v0.33.0: PASS\n")
	require.Contains(t, s, "commit `9e954e900951ba06c0e7b8502a7361ffdbd6f91f`")
	require.Contains(t, s, "| das | pass | native_sample_observed | 0.1 |\n")
	require.Contains(t, s, "| das_recent (informational) | inconclusive | recent_sampling_timeout | 180.0 |\n")
	require.Contains(t, s, "Timings from process start: startup 2.3s, bootstrappers 5/6, sync 239 headers/s.")

	r.Metrics = model.Metrics{}
	require.NotContains(t, Summary(r), "Timings")
}
