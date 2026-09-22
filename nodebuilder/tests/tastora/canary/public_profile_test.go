package canary

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

func TestPublicProfileUsesNativeNetworkDefaults(t *testing.T) {
	image := model.OfficialImageRepository + "@sha256:" + strings.Repeat("a", 64)
	commit := strings.Repeat("0d95dec2", 5)
	p, err := PublicProfile(model.ProfileMocha, image, commit, "v0.33.2-mocha")
	require.NoError(t, err)
	require.Equal(t, "mocha-5", p.Network)
	require.Equal(t, "mocha-5", p.ChainID)
	require.Equal(t, SchemaFor(commit), p.TelemetrySchema)
	require.Equal(t, "v0.33.2-mocha", p.Release)

	p, err = PublicProfile(model.ProfileMainnet, "sha256:"+strings.Repeat("b", 64), commit, "")
	require.NoError(t, err, "a local build runs by image ID")
	require.Equal(t, "celestia", p.Network)

	for name, args := range map[string][4]string{
		"unknown network":   {"arabica", image, commit, ""},
		"foreign digest":    {model.ProfileMocha, "example.invalid/node@sha256:" + strings.Repeat("c", 64), commit, ""},
		"floating tag":      {model.ProfileMocha, model.OfficialImageRepository + ":latest", commit, ""},
		"short commit":      {model.ProfileMocha, image, "0d95dec", ""},
		"malformed release": {model.ProfileMocha, image, commit, "latest"},
		"local profile":     {model.ProfileLocal, image, commit, ""},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := PublicProfile(args[0], args[1], args[2], args[3])
			require.ErrorIs(t, err, ErrUnsupportedProfile)
		})
	}
}
