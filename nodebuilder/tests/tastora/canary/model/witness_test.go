package model

import (
	"strings"
	"testing"
	"time"
)

func TestWitnessMetadata(t *testing.T) {
	cases := map[string]func(*Result){
		"missing": func(r *Result) { r.Witnesses = nil }, "partial": func(r *Result) { r.Witnesses = r.Witnesses[:1] },
		"extra":           func(r *Result) { r.Witnesses = append(r.Witnesses, r.Witnesses[2]) },
		"epoch":           func(r *Result) { r.Witnesses[2].Epoch = 1 },
		"first_epoch":     func(r *Result) { r.Witnesses[0].Epoch = 2 },
		"check":           func(r *Result) { r.Witnesses[1].Check = "cache" },
		"duplicate_check": func(r *Result) { r.Witnesses[1].Check = "restart" },
		"first_check": func(r *Result) {
			r.Witnesses[0].Check, r.Witnesses[1].Check = "das_recent", "das"
			r.Witnesses[0].JobType, r.Witnesses[1].JobType = "recent", "catchup"
		},
		"kind":         func(r *Result) { r.Witnesses[1].EvidenceKind = "counter" },
		"job":          func(r *Result) { r.Witnesses[1].JobType = "cache" },
		"recent_job":   func(r *Result) { r.Witnesses[1].JobType = "catchup" },
		"amount":       func(r *Result) { r.Witnesses[1].SampleCount = 1 },
		"width":        func(r *Result) { r.Witnesses[1].Header.Width = 0 },
		"height":       func(r *Result) { r.Witnesses[1].Header.Height = 0 },
		"hash":         func(r *Result) { r.Witnesses[1].Header.Hash = "invalid" },
		"root":         func(r *Result) { r.Witnesses[1].Header.Root = "invalid" },
		"chain":        func(r *Result) { r.Witnesses[1].Header.ChainID = "wrong" },
		"header_time":  func(r *Result) { r.Witnesses[1].Header.Time = time.Time{} },
		"start":        func(r *Result) { r.Witnesses[1].StartedAt = time.Time{} },
		"order":        func(r *Result) { r.Witnesses[1].CompletedAt = r.Witnesses[1].StartedAt.Add(-time.Second) },
		"duration":     func(r *Result) { r.Witnesses[1].DurationSeconds = -1 },
		"outside":      func(r *Result) { r.Witnesses[2].CompletedAt = r.FinishedAt.Add(time.Second) },
		"before_run":   func(r *Result) { r.Witnesses[0].StartedAt = r.StartedAt.Add(-time.Second) },
		"same_root":    func(r *Result) { r.Witnesses[1].Header.Root = r.Witnesses[0].Header.Root },
		"same_hash":    func(r *Result) { r.Witnesses[2].Header.Hash = r.Witnesses[0].Header.Hash },
		"recent_below": func(r *Result) { r.Witnesses[1].Header.Height = r.Witnesses[0].Header.Height },
		"reversed":     func(r *Result) { r.Witnesses[2].CompletedAt = r.Witnesses[1].CompletedAt },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := completeResult()
			mutate(&r)
			if r.Validate() == nil {
				t.Fatal("invalid witness unexpectedly accepted")
			}
		})
	}
}

func TestPartialWitnessesAllowedOnNegativeOutcome(t *testing.T) {
	r := completeResult()
	r.Witnesses = r.Witnesses[:1]
	r.Checks[5].Outcome = Inconclusive
	r.Checks[6].Outcome = Fail
	r.Outcome = Fail
	if err := r.Validate(); err != nil {
		t.Fatalf("negative run with a first witness rejected: %v", err)
	}
}

func TestWitnessesRemainChronological(t *testing.T) {
	r := completeResult()
	r.Witnesses = []Witness{r.Witnesses[0], r.Witnesses[2], r.Witnesses[1]}
	if r.Validate() == nil {
		t.Fatal("non-chronological witnesses accepted")
	}
}

func TestWitnessRejectsCaseVariantRootReuse(t *testing.T) {
	for _, field := range []string{"root", "hash"} {
		t.Run(field, func(t *testing.T) {
			r := completeResult()
			if field == "root" {
				r.Witnesses[2].Header.Root = strings.ToLower(r.Witnesses[0].Header.Root)
			} else {
				r.Witnesses[2].Header.Hash = strings.ToLower(r.Witnesses[0].Header.Hash)
			}
			before := r.Witnesses[2].Header
			if err := r.Validate(); err == nil || err.Error() != "witness reuses prior data" {
				t.Fatalf("case-variant %s reuse: got %v, want reuse rejection", field, err)
			}
			if r.Witnesses[2].Header != before {
				t.Fatal("validation changed submitted header spelling")
			}
		})
	}
	t.Run("distinct roots and hashes retain spelling", func(t *testing.T) {
		r := completeResult()
		r.Witnesses[2].Header.Root = strings.ToLower(r.Witnesses[2].Header.Root)
		r.Witnesses[2].Header.Hash = strings.ToLower(r.Witnesses[2].Header.Hash)
		before := r.Witnesses[2].Header
		if err := r.Validate(); err != nil {
			t.Fatalf("distinct case-varied header rejected: %v", err)
		}
		if r.Witnesses[2].Header != before {
			t.Fatal("validation changed submitted header spelling")
		}
	})
	t.Run("validate spelling before comparing", func(t *testing.T) {
		r := completeResult()
		r.Witnesses[0].Header.Root = strings.Repeat("G", 64)
		r.Witnesses[2].Header.Root = strings.Repeat("g", 64)
		if err := r.Validate(); err == nil || err.Error() != "invalid witnessed header" {
			t.Fatalf("malformed hex: got %v, want invalid header", err)
		}
	})
}

func TestWitnessRejectsReversedEpochTimes(t *testing.T) {
	r := completeResult()
	cases := []struct {
		name  string
		start time.Time
		valid bool
	}{
		{"reversed", r.StartedAt.Add(30 * time.Second), false},
		{"overlapping", r.Witnesses[1].CompletedAt.Add(-3 * time.Second), false},
		{"same completion boundary", r.Witnesses[1].CompletedAt.Add(-2 * time.Second), false},
		{"just after completion", r.Witnesses[1].CompletedAt.Add(-2*time.Second + time.Nanosecond), true},
		{"original chronology", r.Witnesses[2].StartedAt, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := completeResult()
			last := &r.Witnesses[2]
			last.Header.Time = tc.start.Add(-time.Second)
			last.StartedAt = tc.start
			last.CompletedAt = tc.start.Add(2 * time.Second)
			err := r.Validate()
			if tc.valid {
				if err != nil {
					t.Fatalf("valid restart chronology rejected: %v", err)
				}
			} else if err == nil || err.Error() != "witness completion must follow prior completion" {
				t.Fatalf("impossible restart chronology: got %v, want chronology rejection", err)
			}
		})
	}
}
