package canary

import (
	"time"

	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/availability/light"
)

// PublicProfile binds one public network to an image pinned by ResolveImage.
// Sampling amount and windows are the light node's defaults, which a
// public profile may not change; release is the tag the image was resolved
// from, if any, and is informational.
func PublicProfile(name, image, revision, release string) (model.Profile, error) {
	network := p2p.GetNetwork(name).String()
	p := model.Profile{
		Name:                  name,
		Network:               network,
		ChainID:               network,
		Image:                 image,
		SourceCommit:          revision,
		TelemetrySchema:       SchemaFor(revision),
		Release:               release,
		Bootstrappers:         []string{},
		SampleAmount:          int(light.DefaultSampleAmount),
		SamplingWindowSeconds: int64(availability.SamplingWindow / time.Second),
		StorageWindowSeconds:  int64(availability.StorageWindow / time.Second),
	}
	return p, ValidateProfile(p)
}
