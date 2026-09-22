package telemetry

import (
	"context"
	"encoding/hex"
	"errors"
	"slices"
	"time"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/share"
)

// HeaderLookup resolves an exact header hash. The engine supplies
// validated observations (network heads and the node's own local store).
type HeaderLookup interface {
	GetByHash(context.Context, libhead.Hash) (*header.ExtendedHeader, error)
}

type candidate struct {
	start, completion logRecord
	amount            int
	// noSession marks a completion without a sampling session start. The
	// availability check short-circuits for an empty data square
	// before logging a session, so such a completion is a real DAS worker
	// pass only when the header proves the square is empty.
	noSession bool
}

// candidatesLocked joins each qualifying completion with the latest preceding
// session start for the same data root in the same epoch. A completion with no
// start is returned as a session-less candidate: it is a witness only when the
// header proves an empty square and the request allows it; for a non-empty
// square it is a cache hit or a lost record, never a fresh sampling witness.
// Reported missing=true means evidence for a qualifying completion is absent.
func (c *Collector) candidatesLocked(req WitnessRequest, deadline time.Time) ([]candidate, bool) {
	var out []candidate
	missing := false
	if c.opts.NodeID == "" {
		return nil, true
	}
	b := c.epochs[req.Epoch]
	if b == nil {
		return nil, true
	}
	for _, p := range c.completions {
		end := p.end
		if end.epoch != req.Epoch || end.height <= req.MinHeight {
			continue
		}
		if req.JobType != "" && end.jobType != req.JobType {
			continue
		}
		// zap timestamps truncate to milliseconds. A record written before the
		// process stopped or the phase deadline has ts <= that instant, so the
		// lower edge decides those bounds; the conservative upper edge decides
		// the readiness fence and the window checks.
		upper := end.ts.Add(time.Millisecond)
		if (!deadline.IsZero() && end.ts.After(deadline)) || (!b.ended.IsZero() && end.ts.After(b.ended)) {
			continue
		}
		if !req.NotBefore.IsZero() && !upper.After(req.NotBefore) {
			continue
		}
		start, found, failed := p.start, p.found, p.failed
		if !found {
			if end.ts.Before(b.StartedAt) || (!req.NotBefore.IsZero() && end.ts.Before(req.NotBefore)) {
				continue
			}
			out = append(out, candidate{start: end, completion: end, noSession: true})
			continue
		}
		if failed || start.ts.Before(b.StartedAt) || (!req.NotBefore.IsZero() && start.ts.Before(req.NotBefore)) ||
			start.ts.After(upper) {
			continue
		}
		out = append(
			out,
			candidate{start: start, completion: end, amount: min(c.opts.SampleAmount, end.width*end.width)},
		)
	}
	return out, missing
}

func validateHeader(h *header.ExtendedHeader) (ok bool) {
	// RPC inputs can contain nil nested structures, and header validation assumes
	// some fields are set; a malformed response must be non-PASS, not crash the runner.
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return h != nil && h.Commit != nil && h.DAH != nil && h.ValidatorSet != nil && h.Validate() == nil
}

func matchesHeader(h *header.ExtendedHeader, a candidate, o Options) bool {
	if !validateHeader(h) {
		return false
	}
	r := a.completion
	if h.ChainID() != o.ChainID || h.Height() != r.height || h.Hash().String() != r.hash || h.DAH.String() != r.root ||
		len(h.DAH.RowRoots) != r.width {
		return false
	}
	// A session-backed completion must carry data; a session-less one is only
	// legitimate for the empty square (anything else is a cache hit or loss).
	if share.DataHash(h.DAH.Hash()).IsEmptyEDS() != a.noSession {
		return false
	}
	// The block must exist before its sampling session started.
	if h.Time().After(a.start.ts.Add(o.ClockUncertainty)) {
		return false
	}
	if o.SamplingWindow > 0 {
		if o.ClockUncertainty > o.SamplingWindow {
			return false
		}
		limit := o.SamplingWindow - o.ClockUncertainty
		if a.start.ts.Sub(h.Time()) > limit || r.ts.Add(time.Millisecond).Sub(h.Time()) > limit {
			return false
		}
	}
	return true
}

// WaitWitness waits for a qualifying completion whose header the lookup
// resolves and validates. The caller supplies the phase deadline as the context
// deadline. Aggregate stats and RPC success are never treated as evidence.
func (c *Collector) WaitWitness(ctx context.Context, req WitnessRequest, lookup HeaderLookup) (model.Witness, error) {
	if lookup == nil || req.Epoch < 1 {
		return model.Witness{}, ErrIncompatible
	}
	deadline, _ := ctx.Deadline()
	// A session-less completion whose header resolves to a non-empty square is
	// a lost record or a cache hit; once seen it keeps the phase inconclusive.
	gapSeen := false
	// Headers resolve once per call: a busy node logs thousands of completions
	// and every lookup is an RPC round trip.
	resolved := map[string]*header.ExtendedHeader{}
	unresolved := map[string]bool{}
	for {
		c.mu.Lock()
		if err := c.problemLocked(); err != nil {
			c.mu.Unlock()
			return model.Witness{}, err
		}
		changed := c.changed
		sealed := c.sealed
		options := c.opts
		candidates, missing := c.candidatesLocked(req, deadline)
		c.mu.Unlock()
		if err := ctx.Err(); err != nil {
			if missing || gapSeen {
				return model.Witness{}, errors.Join(ErrEvidenceGap, err)
			}
			return model.Witness{}, errors.Join(ErrNoWitness, err)
		}
		if !req.AllowEmpty {
			// Only a session-backed completion can satisfy this request; the
			// session-less ones matter only for spotting lost records, so they
			// are checked after the candidates that can answer.
			slices.SortStableFunc(candidates, func(a, b candidate) int {
				switch {
				case a.noSession == b.noSession:
					return 0
				case b.noSession:
					return -1
				default:
					return 1
				}
			})
		}
		for _, a := range candidates {
			if a.noSession && !req.AllowEmpty && (gapSeen || unresolved[a.completion.hash]) {
				continue
			}
			h, ok := resolved[a.completion.hash]
			if !ok {
				hash, _ := hex.DecodeString(a.completion.hash)
				var err error
				h, err = lookup.GetByHash(ctx, libhead.Hash(hash))
				if err != nil {
					if a.noSession && !req.AllowEmpty {
						unresolved[a.completion.hash] = true // diagnostic only; do not retry
					}
					continue
				}
				resolved[a.completion.hash] = h
			}
			if !matchesHeader(h, a, options) {
				if a.noSession && validateHeader(h) && !share.DataHash(h.DAH.Hash()).IsEmptyEDS() {
					gapSeen = true // non-empty square with no session start: lost or cached
				}
				continue
			}
			if a.noSession && !req.AllowEmpty {
				continue // an empty square sampled nothing; this request needs a session
			}
			// A completion's pairing depends only on records logged before it, so
			// later records cannot invalidate this witness; only a collector-wide
			// gap or incompatibility can.
			c.mu.Lock()
			problem := c.problemLocked()
			c.mu.Unlock()
			if problem != nil {
				return model.Witness{}, problem
			}
			if err := ctx.Err(); err != nil {
				return model.Witness{}, errors.Join(ErrNoWitness, err)
			}
			return model.Witness{
				Header: model.HeaderRef{
					Height:  h.Height(),
					Hash:    h.Hash().String(),
					Root:    h.DAH.String(),
					ChainID: h.ChainID(),
					Time:    h.Time(),
					Width:   len(h.DAH.RowRoots),
				},
				Epoch:           req.Epoch,
				JobType:         a.completion.jobType,
				SampleCount:     a.amount,
				Empty:           a.noSession,
				StartedAt:       a.start.ts,
				CompletedAt:     a.completion.ts.Add(time.Millisecond),
				DurationSeconds: a.completion.duration,
				EvidenceKind:    model.EvidenceNativeLogs,
			}, nil
		}
		if sealed {
			if missing || gapSeen {
				return model.Witness{}, ErrEvidenceGap
			}
			return model.Witness{}, ErrNoWitness
		}
		timer := time.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
		case <-changed:
		case <-timer.C:
		}
		timer.Stop()
	}
}
