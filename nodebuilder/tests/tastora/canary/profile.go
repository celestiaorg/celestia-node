package canary

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/availability/light"
)

// AdapterVersion names the log adapter implemented by the telemetry
// package: the shape of the share/light and das records it accepts. Bump it
// when those records change.
const AdapterVersion = "v1"

var (
	ErrUnsupportedProfile = errors.New("unsupported canary profile")
	imageDigest           = regexp.MustCompile(`^[^\s@]+@sha256:[a-f0-9]{64}$`)
	imageID               = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	sourceCommit          = regexp.MustCompile(`^[a-f0-9]{40}$`)
	releaseTag            = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[a-z0-9]+)?$`)
)

// SchemaFor names the telemetry schema of a run: the source commit of the node
// under test plus the adapter version. Docker checks that the image's OCI
// revision label equals the commit, and the collector rejects records that do
// not fit the adapter at run time, so a commit that changes the evidence
// loggers cannot pass silently.
func SchemaFor(commit string) string {
	if len(commit) < 7 {
		return ""
	}
	return "celestia-node-" + commit[:7] + "-" + AdapterVersion
}

// validateRun admits a profile together with the phase budgets of one run.
func validateRun(p model.Profile, o RunOptions) error {
	if err := ValidateProfile(p); err != nil {
		return err
	}
	reject := func(why string) error { return fmt.Errorf("%w: %s", ErrUnsupportedProfile, why) }
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,48}$`).MatchString(o.RunID) {
		return reject("invalid run ID")
	}
	if o.HeaderCount <= 0 || o.HeaderCount > 10000 || (p.Name != model.ProfileLocal && o.HeaderCount != 100) {
		return reject("public runs require exactly 100 successors")
	}
	for _, d := range []time.Duration{
		o.RunTimeout, o.BootstrapTimeout, o.HeaderTimeout, o.SamplingTimeout, o.RecentTimeout,
		o.RestartTimeout, o.CleanupTimeout, o.PollInterval, o.HeadTolerance, o.ExitSettle,
	} {
		if d <= 0 {
			return reject("phase durations must be positive")
		}
	}
	if o.ClockUncertainty < 0 {
		return reject("negative clock uncertainty")
	}
	return nil
}

// ValidateProfile checks a profile as written; it does not resolve release pins.
// At run time the pulled image's revision label must match the source commit.
func ValidateProfile(p model.Profile) error {
	reject := func(why string) error { return fmt.Errorf("%w: %s", ErrUnsupportedProfile, why) }
	switch p.Name {
	case model.ProfileMainnet, model.ProfileMocha, model.ProfileLocal:
	default:
		return reject("unknown network profile")
	}
	if p.Network == "" || p.ChainID == "" {
		return reject("missing network/chain identity")
	}
	if !sourceCommit.MatchString(p.SourceCommit) {
		return reject("source commit must be a full lowercase 40-hex revision")
	}
	if p.TelemetrySchema != SchemaFor(p.SourceCommit) {
		return reject("telemetry schema must name the native log adapter of the source commit")
	}
	if p.Release != "" && !releaseTag.MatchString(p.Release) {
		return reject("release must be a native tag such as v0.33.0 or v0.33.1-mocha")
	}
	if !imageDigest.MatchString(p.Image) && !imageID.MatchString(p.Image) {
		return reject("immutable repository digest or exact local image ID required")
	}
	if (p.Name == model.ProfileMainnet || p.Name == model.ProfileMocha) && imageDigest.MatchString(p.Image) &&
		!strings.HasPrefix(p.Image, model.OfficialImageRepository+"@sha256:") {
		return reject("public release digests must come from the official image repository")
	}
	if p.SampleAmount <= 0 || p.SamplingWindowSeconds <= 0 || p.StorageWindowSeconds <= 0 ||
		p.SamplingWindowSeconds > math.MaxInt64/int64(time.Second) ||
		p.StorageWindowSeconds > math.MaxInt64/int64(time.Second) {
		return reject("invalid effective sampling/storage configuration")
	}
	if p.Name != model.ProfileLocal {
		if _, ok := os.LookupEnv("CELESTIA_OVERRIDE_AVAILABILITY_WINDOW"); ok {
			return reject("availability window override forbidden")
		}
		if p.SampleAmount != int(light.DefaultSampleAmount) ||
			p.SamplingWindowSeconds != int64(availability.SamplingWindow/time.Second) ||
			p.StorageWindowSeconds != int64(availability.StorageWindow/time.Second) {
			return reject("public native defaults changed")
		}
	}
	if p.Name == model.ProfileMainnet || p.Name == model.ProfileMocha {
		net := p2p.GetNetwork(p.Name)
		if p.Network != net.String() || p.ChainID != net.String() {
			return reject("public network/chain mismatch")
		}
		if p.CustomNetwork != "" {
			return reject("public custom network forbidden")
		}
		peers, err := p2p.BootstrappersFor(net)
		if err != nil {
			return reject("native bootstrap configuration unavailable")
		}
		var defaults []string
		for _, pi := range peers {
			addrs, err := peer.AddrInfoToP2pAddrs(&pi)
			if err != nil {
				return reject("invalid native bootstrap")
			}
			for _, a := range addrs {
				defaults = append(defaults, a.String())
			}
		}
		if len(p.Bootstrappers) > 0 {
			supplied := slices.Clone(p.Bootstrappers)
			slices.Sort(defaults)
			slices.Sort(supplied)
			if !slices.Equal(defaults, supplied) {
				return reject("public bootstrap overrides forbidden")
			}
		}
	}
	return nil
}
