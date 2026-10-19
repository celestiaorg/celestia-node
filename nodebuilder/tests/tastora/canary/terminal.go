package canary

import (
	"context"
	"time"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/telemetry"
)

type terminalEntry struct {
	check   string
	request telemetry.WitnessRequest
	witness model.Witness
}

// terminalVerification retains each selected witness and its detached header.
// After the last process stopped and the collector was sealed, every witness is
// re-derived from the immutable evidence; a changed witness revokes its PASS.
type terminalVerification struct {
	headers *witnessHeaders
	entries []terminalEntry
}

func (v *terminalVerification) add(check string, req telemetry.WitnessRequest, w model.Witness) {
	v.entries = append(v.entries, terminalEntry{check: check, request: req, witness: w})
}

func (v *terminalVerification) verify(ctx context.Context, c *telemetry.Collector, r *model.Result) {
	if c == nil {
		return
	}
	sealErr := c.Seal(ctx)
	if v.headers != nil {
		v.headers.sealed = true
	}
	for _, entry := range v.entries {
		confirmed, err := c.WaitWitness(ctx, entry.request, v.headers)
		confirmed.Check = entry.witness.Check // the collector does not know check names
		for j := range r.Checks {
			if r.Checks[j].Name != entry.check {
				continue
			}
			// A later lifecycle failure/unsupported outcome remains stronger.
			// Terminal evidence can revoke PASS, never upgrade a negative run.
			if r.Checks[j].Outcome == model.Pass &&
				(sealErr != nil || err != nil || !sameWitness(confirmed, entry.witness)) {
				setCheck(
					r,
					entry.check,
					model.Inconclusive,
					"evidence_changed_after_flush",
					time.Now(),
					c.Diagnostics(),
				)
			}
			if r.Checks[j].Evidence == nil {
				r.Checks[j].Evidence = map[string]any{}
			}
			r.Checks[j].Evidence["terminal_collector"] = c.Diagnostics()
		}
	}
}
