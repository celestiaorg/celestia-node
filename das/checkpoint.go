package das

import (
	"fmt"
)

type checkpoint struct {
	SampleFrom  uint64 `json:"sample_from"`
	NetworkHead uint64 `json:"network_head"`
	// Failed heights will be retried
	Failed map[uint64]int `json:"failed,omitempty"`
	// Workers will resume on restart from previous state
	Workers []workerCheckpoint `json:"workers,omitempty"`
}

// workerCheckpoint will be used to resume worker on restart
type workerCheckpoint struct {
	From    uint64  `json:"from"`
	To      uint64  `json:"to"`
	JobType jobType `json:"job_type"`
}

func newCheckpoint(stats SamplingStats) checkpoint {
	workers := make([]workerCheckpoint, 0, len(stats.Workers))
	for _, w := range stats.Workers {
		// no need to store retry jobs, since they will resume from failed heights map
		if w.JobType == retryJob {
			continue
		}
		workers = append(workers, workerCheckpoint{
			From:    w.Curr,
			To:      w.To,
			JobType: w.JobType,
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
