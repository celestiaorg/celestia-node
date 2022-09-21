package das

// SamplingStats collects information about the DASer process. Currently, there are
// only two sampling routines: the main sampling routine which performs sampling
// over current network headers, and the `catchUp` routine which performs sampling
// over past headers from the last sampled checkpoint.
type SamplingStats struct {
	// all headers before SampledChainHead were successfully sampled
	SampledChainHead uint64 `json:"head_of_sampled_chain"`
	// all headers before CatchupHead were submitted to sampling workers
	CatchupHead uint64 `json:"head_of_catchup"`
	// NetworkHead is the height of the most recent header in the network
	NetworkHead uint64 `json:"network_head_height"`
	// Failed contains all skipped header's heights with corresponding try count
	Failed map[uint64]int `json:"failed,omitempty"`
	// Workers has information about each currently running worker stats
	Workers []WorkerStats `json:"workers,omitempty"`
	// Concurrency currently running parallel workers
	Concurrency int `json:"concurrency"`
	// CatchUpDone indicates whether all known headers are sampled
	CatchUpDone bool `json:"catch_up_done"`
	// IsRunning tracks whether the DASer service is running
	IsRunning bool `json:"is_running"`
}

type WorkerStats struct {
	Curr uint64 `json:"current"`
	From uint64 `json:"from"`
	To   uint64 `json:"to"`

	ErrMsg string `json:"error,omitempty"`
}
