package canary

import (
	"fmt"
	"strings"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

// Summary renders one result as a short Markdown report: the verdict, every
// check with its code and duration, and the timing metrics. It carries no
// evidence, peer identities or error text, so it is safe for a job summary.
func Summary(r model.Result) string {
	var b strings.Builder
	title := r.Profile.Name
	if r.Profile.Release != "" {
		title += " " + r.Profile.Release
	}
	fmt.Fprintf(&b, "### Fresh node canary: %s: %s\n\n", title, strings.ToUpper(string(r.Outcome)))
	fmt.Fprintf(&b, "Image `%s`, commit `%s`.\n\n", r.Profile.Image, r.Profile.SourceCommit)
	b.WriteString("| Check | Outcome | Code | Seconds |\n|---|---|---|---:|\n")
	for _, c := range r.Checks {
		name := c.Name
		if model.IsSoft(c.Name) {
			name += " (informational)"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %.1f |\n", name, c.Outcome, c.Code, c.DurationSeconds)
	}
	var metrics []string
	for _, m := range []struct {
		name  string
		value *float64
	}{
		{"startup", r.Metrics.StartupSeconds},
		{"header sync", r.Metrics.HeaderSyncSeconds},
		{"first sample", r.Metrics.FirstSampleSeconds},
		{"live head sample", r.Metrics.RecentSampleSeconds},
		{"restart resume", r.Metrics.RestartResumeSeconds},
	} {
		if m.value != nil {
			metrics = append(metrics, fmt.Sprintf("%s %.1fs", m.name, *m.value))
		}
	}
	if r.Metrics.BootstrappersConnected != nil && r.Metrics.BootstrappersTotal != nil {
		metrics = append(
			metrics,
			fmt.Sprintf("bootstrappers %d/%d", *r.Metrics.BootstrappersConnected, *r.Metrics.BootstrappersTotal),
		)
	}
	if r.Metrics.SyncHeadersPerSecond != nil {
		metrics = append(metrics, fmt.Sprintf("sync %.0f headers/s", *r.Metrics.SyncHeadersPerSecond))
	}
	if len(metrics) > 0 {
		fmt.Fprintf(&b, "\nTimings from process start: %s.\n", strings.Join(metrics, ", "))
	}
	return b.String()
}
