// Package telemetry turns a light node's own stderr JSON records into
// data-availability-sampling (DAS) evidence. It never retrieves shares itself:
// a witness is the node's "starting sampling session" record joined with its
// "sampled header" completion for the same data root, bound to a header that the
// engine validated independently.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrEvidenceGap  = errors.New("evidence gap")
	ErrIncompatible = errors.New("incompatible native telemetry")
	ErrNoWitness    = errors.New("no autonomous DAS witness")
	ErrSealed       = errors.New("terminal telemetry sealed")
)

// Options bind the collector to one node and one network profile.
type Options struct {
	// Network is an optional profile network pin retained for diagnostics.
	Network          string
	ChainID          string
	NodeID           string
	SampleAmount     int
	SamplingWindow   time.Duration
	ClockUncertainty time.Duration
	MaxRecords       int
}

// Epoch is one process lifetime of the observed node. Restarts open new epochs.
type Epoch struct {
	Number    int
	StartedAt time.Time
}

// WitnessRequest selects which completions may qualify.
type WitnessRequest struct {
	Epoch int
	// NotBefore is a readiness fence: the sampling session must start after it.
	NotBefore time.Time
	// MinHeight excludes completions at or below this height.
	MinHeight uint64
	// JobType restricts the DAS job type ("recent", "catchup", "retry");
	// empty accepts any type.
	JobType string
	// AllowEmpty accepts a completion of a validated empty data square. The
	// light availability check returns before opening a session for an empty
	// square, so the DAS worker logs the completion without a start. Only the
	// first-sample request allows it: a fresh store's only initial job is the
	// tail, and an empty tail leaves the node nothing to sample until a newer
	// head arrives.
	AllowEmpty bool
}

type boot struct {
	Epoch
	ended time.Time
}

type logRecord struct {
	epoch      int
	offset     uint64
	kind       string // kindStart | kindSuccess | kindFailure
	jobType    string
	ts         time.Time
	root, hash string
	height     uint64
	width      int
	duration   float64
}

// Collector must exist before the first process starts and live across
// restarts of the same fresh store. Gaps and incompatible schemas are sticky.
type Collector struct {
	mu           sync.Mutex
	opts         Options
	epochs       map[int]*boot
	logs         []logRecord
	roots        map[string]int
	incompatible bool
	gap          bool
	gapReason    string
	records      int
	unparsed     int
	current      int
	changed      chan struct{}
	sealed       bool
	generation   uint64
	// Pairing indexes per epoch: the latest session start per data root and
	// the offset of the latest failure per height. A completion is paired when
	// it is ingested; records logged after it never change that pairing.
	starts      map[int]map[string]logRecord
	failures    map[int]map[uint64]uint64
	completions []completion
}

// completion is a success record paired with the latest preceding session
// start for its data root in the same epoch.
type completion struct {
	end, start logRecord
	// found reports a preceding start; failed reports a failure for the same
	// height logged after that start.
	found, failed bool
}

const maxLineBytes = 1 << 20

// Record kinds of the log adapter.
const (
	kindStart   = "start"
	kindSuccess = "success"
	kindFailure = "failure"
)

func New(o Options) (*Collector, error) {
	if o.ChainID == "" || o.SampleAmount <= 0 || o.SamplingWindow < 0 || o.ClockUncertainty < 0 || o.MaxRecords < 0 {
		return nil, fmt.Errorf("%w: options", ErrIncompatible)
	}
	if o.MaxRecords == 0 {
		o.MaxRecords = 1 << 20
	}
	return &Collector{
		opts:     o,
		epochs:   make(map[int]*boot),
		roots:    make(map[string]int),
		changed:  make(chan struct{}),
		starts:   make(map[int]map[string]logRecord),
		failures: make(map[int]map[uint64]uint64),
	}, nil
}

func (c *Collector) signalLocked() { c.generation++; close(c.changed); c.changed = make(chan struct{}) }

func (c *Collector) problemLocked() error {
	if c.incompatible {
		return ErrIncompatible
	}
	if c.gap {
		return ErrEvidenceGap
	}
	return nil
}

func (c *Collector) gapLocked(reason string) error {
	if c.sealed {
		return ErrSealed
	}
	c.gap = true
	if c.gapReason == "" {
		c.gapReason = reason
	}
	c.signalLocked()
	return fmt.Errorf("%w: %s", ErrEvidenceGap, reason)
}

func (c *Collector) reserveLocked(n int) error {
	if n > c.opts.MaxRecords-c.records {
		return c.gapLocked("record overflow")
	}
	c.records += n
	return nil
}

// BeginEpoch binds the next sequential epoch to the actual process start time.
func (c *Collector) BeginEpoch(e Epoch) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sealed {
		return ErrSealed
	}
	if e.Number != c.current+1 || e.StartedAt.IsZero() {
		return c.gapLocked("nonsequential epoch")
	}
	if prev := c.epochs[c.current]; prev != nil && (prev.ended.IsZero() || !e.StartedAt.After(prev.ended)) {
		return c.gapLocked("overlapping epoch")
	}
	if err := c.reserveLocked(1); err != nil {
		return err
	}
	c.epochs[e.Number] = &boot{Epoch: e}
	c.current = e.Number
	c.signalLocked()
	return nil
}

// EndEpoch records the observed process stop time of the current epoch.
func (c *Collector) EndEpoch(n int, t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sealed {
		return ErrSealed
	}
	b := c.epochs[n]
	if b == nil || n != c.current || !b.ended.IsZero() || !t.After(b.StartedAt) {
		return c.gapLocked("invalid epoch end")
	}
	for _, r := range c.logs {
		if r.epoch == n && r.ts.After(t) {
			return c.gapLocked("log after epoch end")
		}
	}
	b.ended = t
	c.signalLocked()
	return nil
}

// LogRecord ingests one complete stderr line of the given epoch.
// Lines that are not JSON objects are counted and ignored; malformed records
// from the DAS or light-availability loggers make the run incompatible.
func (c *Collector) LogRecord(epoch int, raw []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sealed {
		return ErrSealed
	}
	if err := c.problemLocked(); err != nil {
		return err
	}
	if len(raw) > maxLineBytes {
		return c.gapLocked("oversize log")
	}
	r, relevant, err := parseLog(raw)
	if errors.Is(err, errNotJSON) {
		c.unparsed++
		return nil
	}
	if err != nil {
		c.incompatible = true
		c.signalLocked()
		return fmt.Errorf("%w: %w", ErrIncompatible, err)
	}
	if !relevant {
		return nil
	}
	b := c.epochs[epoch]
	if b == nil || epoch != c.current || !b.ended.IsZero() || r.ts.Add(time.Millisecond).Before(b.StartedAt) {
		return c.gapLocked("log outside epoch")
	}
	if err := c.reserveLocked(1); err != nil {
		return err
	}
	r.epoch = epoch
	r.offset = uint64(len(c.logs) + 1)
	c.logs = append(c.logs, r)
	c.roots[r.root]++
	c.indexLocked(r)
	c.signalLocked()
	return nil
}

// indexLocked updates the pairing indexes with the record just logged.
func (c *Collector) indexLocked(r logRecord) {
	switch r.kind {
	case kindStart:
		if c.starts[r.epoch] == nil {
			c.starts[r.epoch] = make(map[string]logRecord)
		}
		c.starts[r.epoch][r.root] = r
	case kindFailure:
		if c.failures[r.epoch] == nil {
			c.failures[r.epoch] = make(map[uint64]uint64)
		}
		c.failures[r.epoch][r.height] = r.offset
	case kindSuccess:
		start, found := c.starts[r.epoch][r.root]
		c.completions = append(c.completions, completion{
			end:    r,
			start:  start,
			found:  found,
			failed: found && c.failures[r.epoch][r.height] > start.offset,
		})
	}
}

// MarkGap marks the evidence of the whole run as incomplete. The reason must be
// a fixed category, not raw error text.
func (c *Collector) MarkGap(n int, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.gapLocked(fmt.Sprintf("epoch %d: %.160s", n, reason))
}

// SetNodeID binds the observed node identity once RPC exposes it.
func (c *Collector) SetNodeID(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sealed {
		return ErrSealed
	}
	if id == "" || (c.opts.NodeID != "" && c.opts.NodeID != id) {
		c.incompatible = true
		c.signalLocked()
		return fmt.Errorf("%w: node identity", ErrIncompatible)
	}
	c.opts.NodeID = id
	c.signalLocked()
	return nil
}

// Seal freezes the collector after the last process stopped and its log stream
// was drained. Later records are rejected; the terminal generation is immutable.
func (c *Collector) Seal(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sealed {
		return c.problemLocked()
	}
	if ctx.Err() != nil {
		return c.gapLocked("terminal drain deadline")
	}
	c.sealed = true
	c.signalLocked()
	return c.problemLocked()
}

// Diagnostics returns counters about the ingested records, never log content.
func (c *Collector) Diagnostics() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	success, failure, starts := 0, 0, 0
	var first, last time.Time
	for _, r := range c.logs {
		switch r.kind {
		case kindSuccess:
			success++
			if first.IsZero() || r.ts.Before(first) {
				first = r.ts
			}
			if r.ts.After(last) {
				last = r.ts
			}
		case kindFailure:
			failure++
		case kindStart:
			starts++
		}
	}
	d := map[string]any{
		"sealed":            c.sealed,
		"generation":        c.generation,
		"epochs":            len(c.epochs),
		"log_records":       len(c.logs),
		"roots":             len(c.roots),
		"unparsed_lines":    c.unparsed,
		"incompatible":      c.incompatible,
		"evidence_gap":      c.gap,
		"gap_reason":        c.gapReason,
		"records":           c.records,
		"sampling_sessions": starts,
		"sampled_headers":   success,
		"failed_samples":    failure,
	}
	if !first.IsZero() {
		d["first_sample_at"] = first.UTC()
		d["last_sample_at"] = last.UTC()
	}
	return d
}
