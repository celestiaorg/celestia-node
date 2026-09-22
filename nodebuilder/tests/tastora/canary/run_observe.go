package canary

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/telemetry"
	"github.com/celestiaorg/celestia-node/share"
)

func pause(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func bootstrapIDs(p model.Profile) (map[peer.ID]bool, error) {
	ids := map[peer.ID]bool{}
	if p.Name != model.ProfileLocal {
		peers, err := p2p.BootstrappersFor(p2p.Network(p.Network))
		if err != nil {
			return nil, err
		}
		for _, pi := range peers {
			ids[pi.ID] = true
		}
		return ids, nil
	}
	for _, addr := range p.Bootstrappers {
		a, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		pi, err := peer.AddrInfoFromP2pAddr(a)
		if err != nil {
			return nil, err
		}
		ids[pi.ID] = true
	}
	if len(ids) == 0 {
		return nil, errors.New("local bootstrap identities missing")
	}
	return ids, nil
}

// bootstrapPeer is one bootstrap peer of the network and whether the node was
// connected to it.
type bootstrapPeer struct {
	Peer      string `json:"peer"`
	Connected bool   `json:"connected"`
}

// bootstrapConnectivity lists the profile's bootstrap peers and whether the node
// is connected to each of them right now.
func bootstrapConnectivity(ctx context.Context, r Reader, p model.Profile) ([]bootstrapPeer, int, int, error) {
	ids, err := bootstrapIDs(p)
	if err != nil {
		return nil, 0, 0, err
	}
	ps, err := r.Peers(ctx)
	if err != nil {
		return nil, 0, 0, err
	}
	connected := map[peer.ID]bool{}
	for _, id := range ps {
		if ids[id] {
			connected[id] = true
		}
	}
	out := make([]bootstrapPeer, 0, len(ids))
	for id := range ids {
		out = append(out, bootstrapPeer{Peer: id.String(), Connected: connected[id]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Peer < out[j].Peer })
	return out, len(connected), len(ids), nil
}

// syncRate is the header sync rate over one process lifetime: stored headers
// above the initial tail per second of uptime. Zero when nothing beyond the
// tail is stored yet; absent when the local head cannot be read.
func syncRate(ctx context.Context, r Reader, tail *header.ExtendedHeader, since, now time.Time) *float64 {
	local, err := r.LocalHead(ctx)
	if err != nil || local == nil || tail == nil || !now.After(since) {
		return nil
	}
	synced := 0.0
	if local.Height() > tail.Height() {
		synced = float64(local.Height() - tail.Height())
	}
	rate := synced / now.Sub(since).Seconds()
	return &rate
}

func ready(
	ctx context.Context,
	s Session,
	p model.Profile,
	o RunOptions,
) (Reader, peer.AddrInfo, *header.ExtendedHeader, *header.ExtendedHeader, error) {
	ids, err := bootstrapIDs(p)
	if err != nil {
		return nil, peer.AddrInfo{}, nil, nil, err
	}
	var r Reader
	for ctx.Err() == nil {
		if r == nil {
			r, err = s.RPC(ctx)
		}
		if r != nil && err == nil {
			pi, e := r.Info(ctx)
			if e == nil && pi.ID != "" {
				ps, e := r.Peers(ctx)
				connected := false
				for _, id := range ps {
					if ids[id] {
						connected = true
					}
				}
				if e == nil && connected {
					head, e := r.NetworkHead(ctx)
					if e == nil {
						tail, e := r.Tail(ctx)
						if e == nil {
							return r, pi, head, tail, nil
						}
					}
				}
			}
		}
		// A node that exited will never answer; report it instead of waiting out the phase.
		if info, e := s.Inspect(ctx); e == nil && !info.Running && !info.FinishedAt.IsZero() {
			err = fmt.Errorf("%w with code %d", errProcessExited, info.ExitCode)
			break
		}
		if err = pause(ctx, o.PollInterval); err != nil {
			break
		}
	}
	if r != nil {
		r.Close()
	}
	if errors.Is(err, errProcessExited) {
		return nil, peer.AddrInfo{}, nil, nil, err
	}
	return nil, peer.AddrInfo{}, nil, nil, ctx.Err()
}

// processEvidence records the node process state after a phase did not
// complete, so a report tells a node that died from one that cannot reach the
// network.
func processEvidence(ctx context.Context, s Session) map[string]any {
	info, err := s.Inspect(ctx)
	if err != nil {
		return nil
	}
	return map[string]any{"process": info}
}

func ref(h *header.ExtendedHeader) model.HeaderRef {
	return model.HeaderRef{
		Height:  h.Height(),
		Hash:    h.Hash().String(),
		Root:    h.DAH.String(),
		ChainID: h.ChainID(),
		Time:    h.Time(),
		Width:   len(h.DAH.RowRoots),
	}
}

func processStarted(p model.ProcessInfo, store model.StoreIdentity) bool {
	return p.Running && !p.OOMKilled && !p.ForcedKill && !p.StartedAt.IsZero() && p.ContainerID != "" &&
		p.VolumeID == store.VolumeID &&
		imageID.MatchString(p.ImageID)
}

func sameProcess(a, b model.ProcessInfo) bool {
	return a.ContainerID == b.ContainerID && a.VolumeID == b.VolumeID && a.ImageID == b.ImageID
}

func graceful(a, b model.ProcessInfo) bool {
	return sameProcess(a, b) && a.StartedAt.Equal(b.StartedAt) && !b.Running && !b.OOMKilled && !b.ForcedKill &&
		b.ExitCode == 0 &&
		b.FinishedAt.After(b.StartedAt)
}

func errorOutcome(ctx context.Context, err error) (model.Outcome, string) {
	if errors.Is(err, telemetry.ErrIncompatible) || errors.Is(err, ErrUnsupportedProfile) {
		return model.Unsupported, "incompatible_evidence"
	}
	if errors.Is(err, telemetry.ErrEvidenceGap) {
		return model.Inconclusive, "evidence_gap"
	}
	if errors.Is(err, errProcessExited) {
		return model.Fail, "process_exited"
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return model.Inconclusive, "canceled"
	}
	return model.Fail, "phase_not_completed"
}

// samplingPhase names the codes of one witness wait.
type samplingPhase struct {
	pass, timeout string
	// live requires a head newer than the request's MinHeight to have been
	// observed before a timeout counts as a sampling failure.
	live bool
}

// errProcessExited reports a node process that stopped on its own while the
// engine waited for it.
var errProcessExited = errors.New("node process exited")

var (
	phaseFirstSample   = samplingPhase{pass: "native_sample_observed", timeout: "sampling_timeout"}
	phaseRecentSample  = samplingPhase{pass: "recent_head_sampled", timeout: "recent_sampling_timeout", live: true}
	phaseRestartSample = samplingPhase{pass: "sampling_resumed", timeout: "sampling_timeout"}
)

// headerLookup resolves witness hashes from network heads observed during the
// phase first, then from the node's own local store. Both are validated.
type headerLookup struct {
	heads  *observedHeaders
	reader HeaderReader
	chain  string
}

func (l headerLookup) GetByHash(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	if h, err := l.heads.GetByHash(ctx, hash); err == nil {
		return h, nil
	}
	h, err := l.reader.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if err = validateHeader(h, l.chain); err != nil {
		return nil, err
	}
	return h, nil
}

// waitSampling observes network heads and sampling stats while the collector
// waits for a qualifying completion. Stats are diagnostic only.
func waitSampling(
	ctx context.Context,
	r Reader,
	c *telemetry.Collector,
	req telemetry.WitnessRequest,
	p model.Profile,
	o RunOptions,
	capture *logCapture,
	snapshots *witnessHeaders,
	phase samplingPhase,
) (model.Witness, model.Outcome, string, map[string]any) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	heads := newObservedHeaders(p.ChainID)
	newer, nonempty := false, false
	var stats any
	var observedHead any
	evidence := func() map[string]any {
		return map[string]any{
			"collector":                    c.Diagnostics(),
			"stats_diagnostic_only":        stats,
			"network_head":                 observedHead,
			"newer_head_observed":          newer,
			"newer_nonempty_head_observed": nonempty,
		}
	}
	observe := func() {
		head, err := r.NetworkHead(ctx)
		if err == nil && validateHeader(head, p.ChainID) == nil {
			observedHead = ref(head)
			if head.Height() > req.MinHeight {
				newer = true
				if !share.DataHash(head.DAH.Hash()).IsEmptyEDS() {
					nonempty = true
				}
			}
			_ = heads.observe(head) // bounded phase-local lookup; overflow only disables the shortcut
		}
		if st, err := r.SamplingStats(ctx); err == nil {
			stats = st
		}
	}
	observe()
	lookup := headerLookup{heads: heads, reader: r, chain: p.ChainID}
	type reply struct {
		w   model.Witness
		err error
	}
	ch := make(chan reply, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		w, err := c.WaitWitness(ctx, req, lookup)
		ch <- reply{w, err}
	}()
	defer func() { cancel(); <-done }()
	ticker := time.NewTicker(o.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case got := <-ch:
			ev := evidence()
			if got.err == nil {
				if snapshots == nil {
					ev[evidenceReason] = "missing witness snapshots"
					return model.Witness{}, model.Inconclusive, "native_header_snapshot_unavailable", ev
				}
				if err := snapshots.capture(ctx, lookup, got.w); err != nil {
					ev[evidenceReason] = err.Error()
					return model.Witness{}, model.Inconclusive, "native_header_snapshot_unavailable", ev
				}
				return got.w, model.Pass, phase.pass, ev
			}
			out, code := errorOutcome(ctx, got.err)
			// Refine a missing witness only once the deadline has passed;
			// cancellation and incompatible or incomplete evidence keep their outcome.
			if errors.Is(ctx.Err(), context.DeadlineExceeded) && out == model.Fail &&
				errors.Is(got.err, telemetry.ErrNoWitness) {
				switch {
				case capture == nil || capture.bytes.Load() == 0:
					out, code = model.Inconclusive, "evidence_gap"
				case phase.live && !newer:
					out, code = model.Inconclusive, "no_new_head_observed"
				case phase.live && !nonempty:
					out, code = model.Inconclusive, "no_eligible_nonempty_block"
				default:
					out, code = model.Fail, phase.timeout
				}
			}
			return model.Witness{}, out, code, ev
		case <-ticker.C:
			if info, exited := capture.exited(); exited {
				ev := evidence()
				ev["process"] = info
				return model.Witness{}, model.Fail, "process_exited", ev
			}
			observe()
		}
	}
}
