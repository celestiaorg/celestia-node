//go:build fresh_node_canary

package canary

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	nativedocker "github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/docker"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

// TestFreshNodeCanary starts a brand-new light node against a public network
// and requires every check to pass. It needs Docker and network access and is
// therefore built only with the fresh_node_canary tag.
//
// fail means the node missed a milestone. inconclusive means the run could not
// establish evidence (Docker, host or log stream trouble); it says nothing
// about the node, so the run is repeated once before the test gives up.
//
//	CANARY_NETWORK     mainnet or mocha
//	CANARY_IMAGE       image reference: a tag, a digest or a locally built image
//	CANARY_RELEASE     optional release tag recorded in the report
//	CANARY_REPORT_DIR  optional directory for the JSON result
func TestFreshNodeCanary(t *testing.T) {
	network, ref := os.Getenv("CANARY_NETWORK"), os.Getenv("CANARY_IMAGE")
	networks := []string{model.ProfileMainnet, model.ProfileMocha}
	require.Contains(t, networks, network, "CANARY_NETWORK must be mainnet or mocha")
	require.NotEmpty(t, ref, "CANARY_IMAGE must name the node image under test")

	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Minute)
	defer cancel()

	image, revision, err := nativedocker.ResolveImage(ctx, ref)
	require.NoError(t, err, "resolve image %s", ref)
	profile, err := PublicProfile(network, image, revision, os.Getenv("CANARY_RELEASE"))
	require.NoError(t, err)

	var (
		result  model.Result
		runErr  error
		summary string
	)
	for attempt := 1; attempt <= 2; attempt++ {
		result, runErr = Run(ctx, profile, RunOptions{})
		summary = Summary(result)
		t.Logf("attempt %d\n%s", attempt, summary)
		if result.Outcome != model.Inconclusive {
			break
		}
	}
	if path := os.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = f.WriteString(summary + "\n")
			_ = f.Close()
		}
	}
	if dir := os.Getenv("CANARY_REPORT_DIR"); dir != "" {
		raw, err := json.MarshalIndent(result, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, network+".json"), append(raw, '\n'), 0o644))
	}

	require.NoError(t, runErr)
	require.Equal(t, model.Pass, result.Outcome, "fresh node canary outcome on %s", network)
}
