package model

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestMissingChecksCannotPass(t *testing.T) {
	r := Result{SchemaVersion: "2", RunID: "run-1", Outcome: Pass}
	if err := r.Validate(); err == nil {
		t.Fatal("incomplete report accepted")
	}
}

// completeResult returns a valid synthetic result for schema tests.
func completeResult() Result {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	digest := "sha256:" + strings.Repeat("a", 64)
	r := Result{
		SchemaVersion: "2", RunID: "test-run", NodeID: "test-peer", ImageID: digest,
		Profile: Profile{
			Name: "fixture", Network: "private", ChainID: "private-1", Image: "test/node@" + digest,
			SourceCommit: strings.Repeat("b", 40), TelemetrySchema: "native-v1", SampleAmount: 16,
			SamplingWindowSeconds: 3600, StorageWindowSeconds: 3600,
		},
		StartedAt: start, FinishedAt: start.Add(5 * time.Minute), Outcome: Pass,
	}
	for _, name := range RequiredChecks() {
		r.Checks = append(r.Checks, Check{Name: name, Outcome: Pass, DurationSeconds: 1})
	}
	for i, w := range []struct {
		check string
		epoch int
		job   string
	}{{"das", 1, "catchup"}, {"das_recent", 1, "recent"}, {"restart", 2, "catchup"}} {
		at := start.Add(time.Duration(i+1) * time.Minute)
		r.Witnesses = append(r.Witnesses, Witness{
			Check: w.check, Epoch: w.epoch, JobType: w.job, SampleCount: 16,
			Header: HeaderRef{
				Height: uint64(
					i + 1,
				), Hash: strings.Repeat(string(rune('A'+i)), 64), Root: strings.Repeat(string(rune('D'+i)), 64),
				ChainID: "private-1", Time: at.Add(-time.Second), Width: 4,
			},
			StartedAt: at, CompletedAt: at.Add(2 * time.Second), DurationSeconds: 1.5, EvidenceKind: EvidenceNativeLogs,
		})
	}
	one := 1.0
	r.Metrics = Metrics{
		StartupSeconds:       &one,
		HeaderSyncSeconds:    &one,
		FirstSampleSeconds:   &one,
		RecentSampleSeconds:  &one,
		RestartResumeSeconds: &one,
	}
	return r
}

func TestCompleteResultPasses(t *testing.T) {
	if err := completeResult().Validate(); err != nil {
		t.Fatalf("complete report rejected: %v", err)
	}
}

func TestRequiredChecksAndWitnessChecks(t *testing.T) {
	if len(RequiredChecks()) != 8 || RequiredChecks()[5] != "das_recent" {
		t.Fatalf("unexpected check set %v", RequiredChecks())
	}
	if len(WitnessChecks()) != 3 {
		t.Fatal("three witness checks expected")
	}
	if RequiredChecks()[0] = "mutated"; RequiredChecks()[0] != "fresh_store" {
		t.Fatal("check contract must be immutable")
	}
}

func TestSoftCheckNeverDecidesOutcome(t *testing.T) {
	r := completeResult()
	r.Checks[5].Outcome = Inconclusive
	r.Checks[5].Code = "recent_sampling_timeout"
	r.Witnesses = []Witness{r.Witnesses[0], r.Witnesses[2]}
	if err := r.Validate(); err != nil {
		t.Fatalf("soft live-head miss must not break a PASS: %v", err)
	}
	if Aggregate(r.Checks) != Pass || !IsSoft("das_recent") || IsSoft("das") {
		t.Fatal("soft check leaked into the aggregate")
	}
	r.Checks[5].Outcome = Fail
	if Aggregate(r.Checks) != Pass {
		t.Fatal("soft check failure changed the aggregate")
	}
	r.Checks[4].Outcome = Fail
	if Aggregate(r.Checks) != Fail {
		t.Fatal("hard check failure ignored")
	}
}

func TestMetricsValidation(t *testing.T) {
	r := completeResult()
	bad := -1.0
	r.Metrics.FirstSampleSeconds = &bad
	if r.Validate() == nil {
		t.Fatal("negative metric accepted")
	}
	r = completeResult()
	r.Metrics = Metrics{}
	if err := r.Validate(); err != nil {
		t.Fatalf("absent metrics must be valid: %v", err)
	}
}

func TestEmptyBlockWitnessRules(t *testing.T) {
	r := completeResult()
	r.Witnesses[0].Empty, r.Witnesses[0].SampleCount = true, 0
	if err := r.Validate(); err != nil {
		t.Fatalf("empty first-sample witness rejected: %v", err)
	}
	r.Witnesses[0].SampleCount = 16
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "sample count") {
		t.Fatalf("empty witness with samples accepted: %v", err)
	}
	r = completeResult()
	r.Witnesses[2].Empty, r.Witnesses[2].SampleCount = true, 0
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "first-sample") {
		t.Fatalf("empty restart witness accepted: %v", err)
	}
}

func TestExactCheckSet(t *testing.T) {
	for _, kind := range []string{"duplicate", "unknown", "outcome"} {
		t.Run(kind, func(t *testing.T) {
			r := completeResult()
			switch kind {
			case "duplicate":
				r.Checks[0].Name = r.Checks[1].Name
			case "unknown":
				r.Checks[0].Name = "invented"
			case "outcome":
				r.Checks[0].Outcome = "ok"
			}
			if r.Validate() == nil {
				t.Fatal("invalid check set unexpectedly accepted")
			}
		})
	}
}

func TestReportIdentity(t *testing.T) {
	cases := map[string]func(*Result){
		"schema": func(r *Result) { r.SchemaVersion = "next" }, "run": func(r *Result) { r.RunID = " " },
		"profile": func(r *Result) { r.Profile.Name = "" }, "network": func(r *Result) { r.Profile.Network = "" },
		"chain": func(r *Result) { r.Profile.ChainID = "" }, "node": func(r *Result) { r.NodeID = "" },
		"image":           func(r *Result) { r.Profile.Image = "" },
		"source":          func(r *Result) { r.Profile.SourceCommit = "not-a-commit" },
		"image_id":        func(r *Result) { r.ImageID = "sha256:short" },
		"telemetry":       func(r *Result) { r.Profile.TelemetrySchema = "" },
		"amount":          func(r *Result) { r.Profile.SampleAmount = 0 },
		"sampling_window": func(r *Result) { r.Profile.SamplingWindowSeconds = 0 },
		"storage_window":  func(r *Result) { r.Profile.StorageWindowSeconds = -1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := completeResult()
			mutate(&r)
			if r.Validate() == nil {
				t.Fatal("invalid identity unexpectedly accepted")
			}
		})
	}
}

func TestResultOutcome(t *testing.T) {
	for _, outcome := range []Outcome{Fail, Unsupported, Inconclusive} {
		t.Run(string(outcome), func(t *testing.T) {
			r := completeResult()
			r.Checks[4].Outcome = outcome
			if r.Validate() == nil {
				t.Fatal("contradictory PASS unexpectedly accepted")
			}
			r.Outcome = outcome
			if err := r.Validate(); err != nil {
				t.Fatalf("honest failure rejected: %v", err)
			}
		})
	}
	t.Run("unknown", func(t *testing.T) {
		r := completeResult()
		r.Outcome = "success"
		if r.Validate() == nil {
			t.Fatal("unknown outcome unexpectedly accepted")
		}
	})
	t.Run("false_failure", func(t *testing.T) {
		r := completeResult()
		r.Outcome = Fail
		if r.Validate() == nil {
			t.Fatal("contradictory failure unexpectedly accepted")
		}
	})
	t.Run("priority", func(t *testing.T) {
		r := completeResult()
		r.Checks[0].Outcome = Unsupported
		r.Checks[1].Outcome = Fail
		r.Outcome = Unsupported
		if r.Validate() == nil {
			t.Fatal("masked failure unexpectedly accepted")
		}
		r.Outcome = Fail
		if err := r.Validate(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestReportTiming(t *testing.T) {
	cases := map[string]func(*Result){
		"start":      func(r *Result) { r.StartedAt = time.Time{} },
		"unfinished": func(r *Result) { r.FinishedAt = time.Time{} },
		"inverted":   func(r *Result) { r.FinishedAt = r.StartedAt.Add(-time.Second) },
		"negative":   func(r *Result) { r.Checks[0].DurationSeconds = -1 },
		"nan":        func(r *Result) { r.Checks[0].DurationSeconds = math.NaN() },
		"infinite":   func(r *Result) { r.Checks[0].DurationSeconds = math.Inf(1) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := completeResult()
			mutate(&r)
			if r.Validate() == nil {
				t.Fatal("invalid timing unexpectedly accepted")
			}
		})
	}
}

func TestJSONEvidence(t *testing.T) {
	r := completeResult()
	r.Checks[0].Evidence = map[string]any{"bad": math.NaN()}
	if r.Validate() == nil {
		t.Fatal("nonserializable evidence unexpectedly accepted")
	}
}
