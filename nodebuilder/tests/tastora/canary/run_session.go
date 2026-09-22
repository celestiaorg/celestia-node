package canary

import (
	"context"
	"time"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/telemetry"
)

func seconds(d time.Duration) *float64 {
	v := d.Seconds()
	if v < 0 {
		v = 0
	}
	return &v
}

func executeSession(
	parent context.Context,
	p model.Profile,
	o RunOptions,
	s Session,
	c *telemetry.Collector,
	r *model.Result,
	terminal *terminalVerification,
) {
	ctx, cancel := context.WithTimeout(parent, o.RunTimeout)
	defer cancel()
	start := time.Now()
	bootCtx, bootCancel := context.WithTimeout(ctx, o.BootstrapTimeout)
	defer bootCancel()
	before, e := s.Inspect(bootCtx)
	if e != nil || before.Running || !before.StartedAt.IsZero() {
		out, code := model.Fail, "process_not_fresh"
		if e != nil {
			out, code = errorOutcome(bootCtx, e)
		}
		setCheck(r, model.CheckFreshStore, out, code, start, nil)
		return
	}
	stream, e := s.Logs(ctx)
	if e != nil || stream == nil {
		c.MarkGap(1, "pre-start attach unavailable")
		setCheck(r, model.CheckDAS, model.Inconclusive, "evidence_gap", start, c.Diagnostics())
		return
	}
	defer stream.Close()
	process, e := s.Start(bootCtx)
	if e != nil {
		out, code := errorOutcome(ctx, e)
		setCheck(r, model.CheckBootstrap, out, code, start, nil)
		return
	}
	if !processStarted(process, s.Store()) {
		setCheck(r, model.CheckBootstrap, model.Fail, "invalid_process_identity", start, nil)
		return
	}
	r.ImageID = process.ImageID
	if e = c.BeginEpoch(telemetry.Epoch{Number: 1, StartedAt: process.StartedAt}); e != nil {
		setCheck(r, model.CheckDAS, model.Inconclusive, "evidence_gap", start, c.Diagnostics())
		return
	}
	capture := captureLogs(ctx, stream, c, 1, s, o.ExitSettle)
	defer capture.close()
	reader, pi, head, tail, e := ready(bootCtx, s, p, o)
	if e != nil {
		out, code := errorOutcome(ctx, e)
		setCheck(r, model.CheckBootstrap, out, code, start, processEvidence(ctx, s))
		return
	}
	defer reader.Close()
	r.NodeID = pi.ID.String()
	r.Metrics.StartupSeconds = seconds(time.Since(process.StartedAt))
	bootstrapEvidence := map[string]any{
		"node_id":         r.NodeID,
		"process":         process,
		"startup_seconds": *r.Metrics.StartupSeconds,
	}
	// Readiness needs one connected bootstrap peer; the full list is reported as
	// a metric and does not affect the outcome.
	if peers, connected, total, e := bootstrapConnectivity(bootCtx, reader, p); e == nil {
		r.Metrics.BootstrappersConnected, r.Metrics.BootstrappersTotal = &connected, &total
		bootstrapEvidence["bootstrappers"] = peers
	}
	setCheck(r, model.CheckBootstrap, model.Pass, "native_bootstrap_connected", start, bootstrapEvidence)
	if e = c.SetNodeID(r.NodeID); e != nil {
		out, code := errorOutcome(ctx, e)
		setCheck(r, model.CheckDAS, out, code, start, c.Diagnostics())
		return
	}
	start = time.Now()
	storageWindow := time.Duration(p.StorageWindowSeconds) * time.Second
	if e = ValidateHeadTail(head, tail, p.ChainID, start, storageWindow, o.HeadTolerance, o.ClockUncertainty); e != nil {
		setCheck(
			r,
			model.CheckHeadTail,
			model.Fail,
			"invalid_head_tail",
			start,
			headTailEvidence(e, head, tail, p.StorageWindowSeconds),
		)
		return
	}
	setCheck(
		r,
		model.CheckHeadTail,
		model.Pass,
		"native_timestamp_window",
		start,
		map[string]any{
			"head":                    ref(head),
			"initial_tail":            ref(tail),
			"storage_window_seconds":  p.StorageWindowSeconds,
			"sampling_window_seconds": p.SamplingWindowSeconds,
		},
	)
	start = time.Now()
	hctx, hcancel := context.WithTimeout(ctx, o.HeaderTimeout)
	var hs []*header.ExtendedHeader
	for hctx.Err() == nil {
		hs, e = LocalSuccessors(hctx, reader, tail, o.HeaderCount, p.ChainID)
		if e == nil {
			break
		}
		if _, exited := capture.exited(); exited {
			e = errProcessExited
			break
		}
		if pause(hctx, o.PollInterval) != nil {
			break
		}
	}
	hcancel()
	if e != nil || len(hs) != o.HeaderCount {
		out, code := errorOutcome(ctx, e)
		setCheck(r, model.CheckHeaderSync, out, code, start, processEvidence(ctx, s))
		return
	}
	r.Metrics.HeaderSyncSeconds = seconds(time.Since(process.StartedAt))
	setCheck(
		r,
		model.CheckHeaderSync,
		model.Pass,
		"local_adjacent_successors",
		start,
		map[string]any{
			"count":               len(hs),
			"initial_tail":        ref(tail),
			"last":                ref(hs[len(hs)-1]),
			"header_sync_seconds": *r.Metrics.HeaderSyncSeconds,
		},
	)

	// das: the first sampling completion of any job type.
	// A fresh store's only initial job is the tail (the DASer checkpoint
	// starts with head == tail). When that block is empty the availability
	// check returns before any session and the node has nothing else to
	// sample until a newer head arrives by gossip, so the empty completion is
	// accepted here, flagged, and the restart witness still has to sample.
	start = time.Now()
	snapshots := &witnessHeaders{}
	terminal.headers = snapshots
	firstReq := telemetry.WitnessRequest{Epoch: 1, AllowEmpty: true}
	dctx, dcancel := context.WithTimeout(ctx, o.SamplingTimeout)
	first, out, code, evidence := waitSampling(dctx, reader, c, firstReq, p, o, capture, snapshots, phaseFirstSample)
	dcancel()
	if out == model.Pass {
		first.Check = model.CheckDAS
		if first.Empty {
			code = "native_empty_block_completed"
			evidence["empty_block"] = true
		}
		r.Metrics.FirstSampleSeconds = seconds(first.CompletedAt.Sub(process.StartedAt))
		evidence["witness"] = first
		evidence["first_sample_seconds"] = *r.Metrics.FirstSampleSeconds
	}
	setCheck(r, model.CheckDAS, out, code, start, evidence)
	if out != model.Pass {
		return
	}
	terminal.add(model.CheckDAS, firstReq, first)
	r.Witnesses = append(r.Witnesses, first)

	// A live head produced after the node started, delivered by header gossip
	// and sampled as a "recent" job: the node follows the chain. How
	// quickly a brand-new peer identity joins the header gossip mesh varied
	// from three to more than ten minutes in live runs, so this is a soft
	// check: bounded, published with its timing, never part of the verdict.
	start = time.Now()
	recentReq := telemetry.WitnessRequest{
		Epoch:     1,
		JobType:   model.JobRecent,
		MinHeight: max(head.Height(), first.Header.Height),
		NotBefore: first.CompletedAt,
	}
	rctx, rcancel := context.WithTimeout(ctx, o.RecentTimeout)
	recent, out, code, evidence := waitSampling(rctx, reader, c, recentReq, p, o, capture, snapshots, phaseRecentSample)
	rcancel()
	evidence["bootstrap_head_height"] = head.Height()
	evidence["soft_check"] = true
	switch out {
	case model.Pass:
		recent.Check = model.CheckDASRecent
		r.Metrics.RecentSampleSeconds = seconds(recent.CompletedAt.Sub(process.StartedAt))
		evidence["witness"] = recent
		evidence["recent_sample_seconds"] = *r.Metrics.RecentSampleSeconds
		terminal.add(model.CheckDASRecent, recentReq, recent)
		r.Witnesses = append(r.Witnesses, recent)
	case model.Fail:
		out = model.Inconclusive // not observed within the window; never a verdict
	}
	setCheck(r, model.CheckDASRecent, out, code, start, evidence)

	// Header sync rate over the first process lifetime, read before the stop.
	r.Metrics.SyncHeadersPerSecond = syncRate(ctx, reader, tail, process.StartedAt, time.Now())

	// A successful API call does not prove a restart: compare the identities of the
	// stopped and the started process.
	start = time.Now()
	restartCtx, restartCancel := context.WithTimeout(ctx, o.RestartTimeout)
	defer restartCancel()
	if info, exited := capture.exited(); exited {
		setCheck(r, model.CheckRestart, model.Fail, "process_exited", start, map[string]any{"process": info})
		return
	}
	capture.expected.Store(true)
	stopped, e := s.Stop(restartCtx)
	if e == nil {
		var inspected model.ProcessInfo
		inspected, e = s.Inspect(restartCtx)
		if e == nil && inspected != stopped {
			setCheck(r, model.CheckRestart, model.Fail, "stop_inspect_mismatch", start, nil)
			return
		}
	}
	if e != nil || !graceful(process, stopped) {
		out, code := model.Fail, "not_graceful"
		if e != nil {
			out, code = errorOutcome(restartCtx, e)
		}
		setCheck(r, model.CheckRestart, out, code, start, nil)
		return
	}
	capture.drain(restartCtx)
	if e = c.EndEpoch(1, stopped.FinishedAt); e != nil {
		setCheck(r, model.CheckRestart, model.Inconclusive, "evidence_gap", start, c.Diagnostics())
		return
	}
	reader.Close()
	stream2, e := s.Logs(ctx)
	if e != nil || stream2 == nil {
		c.MarkGap(2, "pre-restart attach unavailable")
		setCheck(r, model.CheckRestart, model.Inconclusive, "evidence_gap", start, c.Diagnostics())
		return
	}
	defer stream2.Close()
	second, e := s.Start(restartCtx)
	if e != nil || !processStarted(second, s.Store()) || !sameProcess(process, second) ||
		!second.StartedAt.After(stopped.FinishedAt) {
		out, code := model.Fail, "restart_identity_changed"
		if e != nil {
			out, code = errorOutcome(restartCtx, e)
		}
		setCheck(r, model.CheckRestart, out, code, start, nil)
		return
	}
	if e = c.BeginEpoch(telemetry.Epoch{Number: 2, StartedAt: second.StartedAt}); e != nil {
		setCheck(r, model.CheckRestart, model.Inconclusive, "evidence_gap", start, c.Diagnostics())
		return
	}
	cap2 := captureLogs(ctx, stream2, c, 2, s, o.ExitSettle)
	defer cap2.close()
	reader2, pi2, _, _, e := ready(restartCtx, s, p, o)
	if e != nil {
		out, code := errorOutcome(ctx, e)
		setCheck(r, model.CheckRestart, out, code, start, processEvidence(ctx, s))
		return
	}
	defer reader2.Close()
	if pi2.ID != pi.ID {
		setCheck(r, model.CheckRestart, model.Fail, "node_id_changed", start, nil)
		return
	}
	preserved, e := reader2.GetByHash(restartCtx, hs[len(hs)-1].Hash())
	if e != nil || validateHeader(preserved, p.ChainID) != nil ||
		preserved.Hash().String() != hs[len(hs)-1].Hash().String() {
		out, code := model.Fail, "retained_header_changed"
		if e != nil {
			out, code = errorOutcome(restartCtx, e)
		}
		setCheck(r, model.CheckRestart, out, code, start, nil)
		return
	}
	fence := time.Now().UTC()
	// Sampling resumed: any completion of a new session after the fence.
	req := telemetry.WitnessRequest{Epoch: 2, NotBefore: fence}
	sampleCtx, sampleCancel := context.WithTimeout(restartCtx, o.SamplingTimeout)
	witness, out, code, evidence := waitSampling(sampleCtx, reader2, c, req, p, o, cap2, snapshots, phaseRestartSample)
	sampleCancel()
	evidence["fence"] = fence
	evidence["before"] = process
	evidence["stopped"] = stopped
	evidence["after"] = second
	evidence["retained_header"] = ref(preserved)
	if out == model.Pass {
		witness.Check = model.CheckRestart
		r.Metrics.RestartResumeSeconds = seconds(witness.CompletedAt.Sub(second.StartedAt))
		evidence["witness"] = witness
		evidence["restart_resume_seconds"] = *r.Metrics.RestartResumeSeconds
	}
	setCheck(r, model.CheckRestart, out, code, start, evidence)
	if out != model.Pass {
		return
	}
	terminal.add(model.CheckRestart, req, witness)
	cap2.expected.Store(true)
	final, e := s.Stop(restartCtx)
	if e != nil || !graceful(second, final) {
		out, code := model.Fail, "final_stop_not_graceful"
		if e != nil {
			out, code = errorOutcome(restartCtx, e)
		}
		setCheck(r, model.CheckRestart, out, code, start, evidence)
		return
	}
	cap2.drain(restartCtx)
	if e = c.EndEpoch(2, final.FinishedAt); e != nil {
		setCheck(r, model.CheckRestart, model.Inconclusive, "evidence_gap", start, c.Diagnostics())
		return
	}
	r.Witnesses = append(r.Witnesses, witness)
}
