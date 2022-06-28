package das

import (
	"sync"
	"time"
)

// state collects information about the DASer process. Currently, there are
// only two sampling routines: the main sampling routine which performs sampling
// over current network headers, and the `catchUp` routine which performs sampling
// over past headers from the last sampled checkpoint.
type state struct {
	sampleLk sync.RWMutex
	sample   RoutineState // tracks information related to the main sampling routine

	catchUpLk sync.RWMutex
	catchUp   JobInfo // tracks information related to the `catchUp` routine
}

// RoutineState contains important information about the state of a
// current sampling routine.
type RoutineState struct {
	// reports if an error has occurred during the routine's
	// sampling process
	Error error `json:"error"`
	// tracks whether routine is running
	IsRunning uint64 `json:"is_running"`
	// tracks the latest successfully sampled height of the routine
	LatestSampledHeight uint64 `json:"latest_sampled_height"`
	// tracks the square width of the latest successfully sampled
	// height of the routine
	LatestSampledSquareWidth uint64 `json:"latest_sampled_square_width"`
}

// JobInfo contains information about a catchUp job.
type JobInfo struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Error error     `json:"error"`

	ID     uint64 `json:"id"`
	Height uint64 `json:"height"`
	From   uint64 `json:"from"`
	To     uint64 `json:"to"`
}

func (ji JobInfo) Finished() bool {
	return ji.To == ji.Height
}

func (ji JobInfo) Duration() time.Duration {
	return ji.End.Sub(ji.Start)
}
