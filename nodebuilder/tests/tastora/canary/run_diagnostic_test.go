package canary

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

func TestEngineNativeMochaDiagnostic(t *testing.T) {
	image := os.Getenv("CFN_ENGINE_DIAGNOSTIC_IMAGE")
	if image == "" {
		t.Skip("explicit locally built source image required")
	}
	p := localProfile()
	p.Name = model.ProfileMocha
	p.Network = p2p.Mocha.String()
	p.ChainID = p2p.Mocha.String()
	p.Image = image
	require.NoError(t, ValidateProfile(p))
	o := RunOptions{
		RunID:            "engine-native-mocha",
		RunTimeout:       120 * time.Second,
		BootstrapTimeout: 60 * time.Second,
		HeaderTimeout:    20 * time.Second,
		SamplingTimeout:  15 * time.Second,
		RestartTimeout:   30 * time.Second,
		CleanupTimeout:   30 * time.Second,
		PollInterval:     time.Second,
		ClockUncertainty: time.Second,
		HeadTolerance:    2 * time.Minute,
	}
	r, err := Run(context.Background(), p, o)
	raw, marshalErr := json.MarshalIndent(r, "", "  ")
	require.NoError(t, marshalErr)
	if path := os.Getenv("CFN_ENGINE_DIAGNOSTIC_RESULT"); path != "" {
		require.NoError(t, os.WriteFile(path, append(raw, '\n'), 0o600))
	}
	t.Logf("NATIVE_RESULT=%s", raw)
	require.NoError(t, err)
	require.NoError(t, r.Validate())
	require.Equal(t, model.Pass, checkNamed(r, "fresh_store").Outcome)
	require.Equal(t, model.Pass, checkNamed(r, "cleanup").Outcome)
	require.Equal(t, image, r.ImageID, "diagnostic must exercise the real image")
	t.Logf("DIAGNOSTIC_OUTCOME=%s (nil error/test success is not canary PASS)", r.Outcome)
}
