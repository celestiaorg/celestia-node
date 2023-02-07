package das

import (
	"fmt"
)

type checkpoint struct {
	SampleFrom  uint64 `json:"sample_from"`
	NetworkHead uint64 `json:"network_head"`
	// Failed will be prioritized on restart
	Failed map[uint64]int `json:"failed,omitempty"`
	// Workers will resume on restart from previous state
	Workers []workerCheckpoint `json:"workers,omitempty"`
}

// workerCheckpoint will be used to resume worker on restart
type workerCheckpoint struct {
	From uint64 `json:"from"`
	To   uint64 `json:"to"`
}

func newCheckpoint(stats SamplingStats) checkpoint {
	workers := make([]workerCheckpoint, 0, len(stats.Workers))
	for _, w := range stats.Workers {
		workers = append(workers, workerCheckpoint{
			From: w.Curr,
			To:   w.To,
		})
	}
	return checkpoint{
		SampleFrom:  stats.CatchupHead + 1,
		NetworkHead: stats.NetworkHead,
		Failed:      stats.Failed,
		Workers:     workers,
	}
}

func (c checkpoint) String() string {
	str := fmt.Sprintf("SampleFrom: %v, NetworkHead: %v", c.SampleFrom, c.NetworkHead)

	if len(c.Workers) > 0 {
		str += fmt.Sprintf(", Workers: %v", len(c.Workers))
	}

	if len(c.Failed) > 0 {
		str += fmt.Sprintf("\nFailed: %v", c.Failed)
	}

	return str
}

// totalSampled returns the total amount of sampled headers
func (c checkpoint) totalSampled() uint64 {
	totalInProgress := 0
	for _, w := range c.Workers {
		totalInProgress += int(w.To-w.From) + 1
	}
	return uint64(int(c.SampleFrom) - totalInProgress - len(c.Failed))
}
