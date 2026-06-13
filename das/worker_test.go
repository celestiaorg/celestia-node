package das

import (
	"fmt"
	"sync"
	"testing"
)

// TestWorker_getStateConcurrentRead verifies that a workerState snapshot
// returned by getState can be read concurrently with the worker mutating its
// `failed` map.
//
// Before the fix, getState returned workerState by value, but the embedded
// `failed` map was shared by reference with the live worker. Iterating that map
// in the coordinator's stats path (unsafeStats) while the worker kept writing to
// it under its lock triggered a fatal "concurrent map iteration and map write".
// This test reproduces that access pattern and must run cleanly (also under
// -race) after the fix deep-copies the map.
func TestWorker_getStateConcurrentRead(t *testing.T) {
	w := &worker{
		state: workerState{
			result: result{
				job:    job{from: 1, to: 1000},
				failed: make(map[uint64]int),
			},
		},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// writer: record a failure per height, mutating `failed` under w.lock,
	// exactly as setResult does during sampling.
	go func() {
		defer wg.Done()
		for h := uint64(1); h <= 1000; h++ {
			w.setResult(h, fmt.Errorf("height %d failed", h))
		}
	}()

	// reader: snapshot the state and iterate `failed`, as the coordinator's
	// unsafeStats does, without holding the worker lock.
	go func() {
		defer wg.Done()
		for range 1000 {
			st := w.getState()
			total := 0
			for _, count := range st.failed {
				total += count
			}
			_ = total
		}
	}()

	wg.Wait()
}
