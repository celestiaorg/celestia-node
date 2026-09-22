package canary

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/share/availability"
)

// fixtureCommit is the source commit the profile fixtures pin.
const fixtureCommit = "9e954e900951ba06c0e7b8502a7361ffdbd6f91f"

func localProfile() model.Profile {
	return model.Profile{
		Name:                  "local",
		Network:               "test",
		ChainID:               "test",
		Image:                 "fixture.invalid/node@sha256:" + strings.Repeat("a", 64),
		SourceCommit:          fixtureCommit,
		TelemetrySchema:       "celestia-node-9e954e9-v1",
		SampleAmount:          16,
		SamplingWindowSeconds: int64(availability.SamplingWindow.Seconds()),
		StorageWindowSeconds:  int64(availability.StorageWindow.Seconds()),
	}
}

// publicProfile is a public-network tuple with an official repository digest.
func publicProfile(name string) model.Profile {
	p := localProfile()
	p.Name = name
	p.Network = p2p.GetNetwork(name).String()
	p.ChainID = p.Network
	p.Image = model.OfficialImageRepository + "@sha256:" + strings.Repeat("c", 64)
	return p
}

func TestPublicProfilesRejectAlteredNativeDefaults(t *testing.T) {
	p := publicProfile(model.ProfileMainnet)
	require.NoError(t, ValidateProfile(p))
	for _, change := range []func(*model.Profile){
		func(p *model.Profile) { p.Network = p2p.Mocha.String() },
		func(p *model.Profile) { p.SamplingWindowSeconds-- },
		func(p *model.Profile) { p.StorageWindowSeconds++ },
		func(p *model.Profile) { p.SampleAmount = 1 },
		func(p *model.Profile) { p.Bootstrappers = []string{"/ip4/127.0.0.1/tcp/2121"} },
		func(p *model.Profile) { p.CustomNetwork = "injected" },
	} {
		q := p
		change(&q)
		require.Error(t, ValidateProfile(q))
	}
	t.Setenv("CELESTIA_OVERRIDE_AVAILABILITY_WINDOW", "1s")
	require.Error(t, ValidateProfile(p))
}

func TestProfileAcceptsExactLocalImageIDWithoutWeakeningSource(t *testing.T) {
	p := localProfile()
	p.Image = "sha256:" + strings.Repeat("b", 64)
	require.NoError(t, ValidateProfile(p))
	for _, image := range []string{
		"sha256:" + strings.Repeat("b", 63),
		"sha256:" + strings.Repeat("B", 64),
		"sha256:latest",
		"node:latest",
	} {
		q := p
		q.Image = image
		require.ErrorIs(t, ValidateProfile(q), ErrUnsupportedProfile)
	}
	p.SourceCommit = fixtureCommit[:7]
	require.ErrorIs(t, ValidateProfile(p), ErrUnsupportedProfile)
}

func TestProfilesRequireExplicitSupportedIdentity(t *testing.T) {
	p := localProfile()
	require.NoError(t, ValidateProfile(p))
	for _, change := range []func(*model.Profile){
		func(p *model.Profile) { p.Image = "node:latest" },
		func(p *model.Profile) { p.SourceCommit = fixtureCommit[:7] },
		func(p *model.Profile) { p.TelemetrySchema = "unknown" },
		func(p *model.Profile) { p.Name = "arabica" },
		func(p *model.Profile) { p.SampleAmount = 0 },
		func(p *model.Profile) { p.SamplingWindowSeconds = 0 },
		func(p *model.Profile) { p.StorageWindowSeconds = 1 << 62 },
	} {
		p := localProfile()
		change(&p)
		require.True(t, errors.Is(ValidateProfile(p), ErrUnsupportedProfile))
	}
}

func TestPublicNetworksRejectPrivateOverrides(t *testing.T) {
	for _, name := range []string{model.ProfileMainnet, model.ProfileMocha} {
		t.Run(name, func(t *testing.T) {
			p := publicProfile(name)
			require.NoError(t, ValidateProfile(p))
			p.CustomNetwork = "private-1::/ip4/192.0.2.1/tcp/2121/p2p/12D3KooWSqZaLcn5Guypo2mrHr297YPJnV8KMEMXNjs3qAS8msw8"
			require.ErrorIs(t, ValidateProfile(p), ErrUnsupportedProfile)
			p.CustomNetwork = ""
			p.Bootstrappers = []string{
				"/ip4/192.0.2.1/tcp/2121/p2p/12D3KooWSqZaLcn5Guypo2mrHr297YPJnV8KMEMXNjs3qAS8msw8",
			}
			require.ErrorIs(t, ValidateProfile(p), ErrUnsupportedProfile)
		})
	}
}

// releaseProfileForTest is a Mainnet v0.33.0 tuple: the multi-platform index
// digest and the commit of the release tag.
func releaseProfileForTest() model.Profile {
	return model.Profile{
		Name:    "mainnet",
		Network: "celestia",
		ChainID: "celestia",
		Image: model.OfficialImageRepository +
			"@sha256:ca3cb74e5256fd9bc96fa18b7fea1a8e6dfd692398768efffd6141e54f4c941c",
		SourceCommit:          fixtureCommit,
		TelemetrySchema:       "celestia-node-9e954e9-v1",
		Release:               "v0.33.0",
		SampleAmount:          16,
		SamplingWindowSeconds: 604800,
		StorageWindowSeconds:  608400,
	}
}

// Any release commit is admitted when the tuple is internally consistent:
// the adapter schema names the commit, the digest comes from the official
// repository, sampling defaults and network identity are untouched. Docker
// still verifies the image revision label against the commit at run time.
func TestReleaseProfileSourceBinding(t *testing.T) {
	p := releaseProfileForTest()
	require.NoError(t, ValidateProfile(p))
	other := strings.Repeat("4cbeb21d", 5)
	for name, change := range map[string]func(*model.Profile){
		"newer release commit": func(p *model.Profile) {
			p.SourceCommit, p.TelemetrySchema, p.Release = other, SchemaFor(other), "v0.33.1-mocha"
		},
		"other official digest": func(p *model.Profile) {
			p.Image = "ghcr.io/celestiaorg/celestia-node@sha256:" + strings.Repeat("a", 64)
		},
		"source build image ID": func(p *model.Profile) { p.Image = "sha256:" + strings.Repeat("b", 64) },
		"no release tag":        func(p *model.Profile) { p.Release = "" },
		"mocha release": func(p *model.Profile) {
			mocha := p2p.Mocha.String()
			p.Name, p.Network, p.ChainID, p.Release = model.ProfileMocha, mocha, mocha, "v0.33.1-mocha"
		},
	} {
		t.Run("accepts "+name, func(t *testing.T) { q := p; change(&q); require.NoError(t, ValidateProfile(q)) })
	}
	for name, change := range map[string]func(*model.Profile){
		"source prefix":         func(p *model.Profile) { p.SourceCommit = fixtureCommit[:7] },
		"uppercase source":      func(p *model.Profile) { p.SourceCommit = strings.ToUpper(p.SourceCommit) },
		"schema of another":     func(p *model.Profile) { p.TelemetrySchema = SchemaFor(other) },
		"unknown adapter":       func(p *model.Profile) { p.TelemetrySchema = "celestia-node-9e954e9-v2" },
		"commit without schema": func(p *model.Profile) { p.SourceCommit = other },
		"tag image":             func(p *model.Profile) { p.Image = "ghcr.io/celestiaorg/celestia-node:v0.33.0" },
		"other repository": func(p *model.Profile) {
			p.Image = strings.Replace(p.Image, "ghcr.io/celestiaorg", "example.invalid", 1)
		},
		"bare release":       func(p *model.Profile) { p.Release = "0.33.0" },
		"floating release":   func(p *model.Profile) { p.Release = "latest" },
		"uppercase release":  func(p *model.Profile) { p.Release = "v0.33.1-Mocha" },
		"network":            func(p *model.Profile) { p.Network = p2p.Mocha.String() },
		"chain":              func(p *model.Profile) { p.ChainID = p2p.Mocha.String() },
		"sample amount":      func(p *model.Profile) { p.SampleAmount++ },
		"sampling window":    func(p *model.Profile) { p.SamplingWindowSeconds-- },
		"storage window":     func(p *model.Profile) { p.StorageWindowSeconds++ },
		"custom network":     func(p *model.Profile) { p.CustomNetwork = "celestia::" },
		"bootstrap override": func(p *model.Profile) { p.Bootstrappers = []string{"/ip4/127.0.0.1/tcp/2121"} },
	} {
		t.Run(
			"rejects "+name,
			func(t *testing.T) { q := p; change(&q); require.ErrorIs(t, ValidateProfile(q), ErrUnsupportedProfile) },
		)
	}
}

func TestSchemaForNamesTheAdapterOfACommit(t *testing.T) {
	require.Equal(t, "celestia-node-9e954e9-v1", SchemaFor("9e954e900951ba06c0e7b8502a7361ffdbd6f91f"))
	require.Equal(t, "celestia-node-4cbeb21-v1", SchemaFor(strings.Repeat("4cbeb21d", 5)))
	require.Equal(t, "", SchemaFor("abc"))
}
