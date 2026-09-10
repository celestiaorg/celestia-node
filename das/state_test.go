package das

import (
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_retryJobGroupsAdjacentFailures(t *testing.T) {
	now := time.Now()
	ready := now.Add(-time.Hour)
	delayed := now.Add(time.Hour)
	state := coordinatorState{
		samplingRange: 3,
		failed: map[uint64]retryAttempt{
			10: {count: 1, after: ready},
			11: {count: 2, after: ready},
			12: {count: 3, after: ready},
			13: {count: 4, after: delayed},
			14: {count: 5, after: ready},
			15: {count: 6, after: ready},
		},
		inRetry: make(map[uint64]retryAttempt),
	}

	jobs := make([]job, 0, 2)
	for range 2 {
		got, found := state.retryJob()
		assert.True(t, found)
		jobs = append(jobs, got)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].from < jobs[j].from })
	assert.Equal(t, retryJob, jobs[0].jobType)
	assert.Equal(t, uint64(10), jobs[0].from)
	assert.Equal(t, uint64(12), jobs[0].to)
	assert.Equal(t, retryJob, jobs[1].jobType)
	assert.Equal(t, uint64(14), jobs[1].from)
	assert.Equal(t, uint64(15), jobs[1].to)

	got, found := state.retryJob()
	assert.False(t, found)
	assert.Equal(t, job{}, got)
	assert.Equal(t, map[uint64]retryAttempt{13: {count: 4, after: delayed}}, state.failed)
	assert.Equal(t, map[uint64]retryAttempt{
		10: {count: 1, after: ready},
		11: {count: 2, after: ready},
		12: {count: 3, after: ready},
		14: {count: 5, after: ready},
		15: {count: 6, after: ready},
	}, state.inRetry)

	state.handleRetryResult(result{
		job:    jobs[0],
		failed: map[uint64]int{11: 1},
	})
	assert.Equal(t, map[uint64]retryAttempt{
		11: {count: 3, after: ready},
		13: {count: 4, after: delayed},
	}, state.failed)
	assert.Equal(t, map[uint64]retryAttempt{
		14: {count: 5, after: ready},
		15: {count: 6, after: ready},
	}, state.inRetry)

	limited := coordinatorState{
		samplingRange: 2,
		failed: map[uint64]retryAttempt{
			20: {after: ready},
			21: {after: ready},
			22: {after: ready},
			23: {after: ready},
		},
		inRetry: make(map[uint64]retryAttempt),
	}
	limitedJob, found := limited.retryJob()
	assert.True(t, found)
	assert.Equal(t, uint64(1), limitedJob.to-limitedJob.from)
	assert.Len(t, limited.failed, 2)
	assert.Len(t, limited.inRetry, 2)
}

func Test_coordinatorStats(t *testing.T) {
	tests := []struct {
		name  string
		state *coordinatorState
		want  SamplingStats
	}{
		{
			"basic",
			&coordinatorState{
				inProgress: map[int]func() workerState{
					1: func() workerState {
						return workerState{
							result: result{
								job: job{
									jobType: recentJob,
									from:    21,
									to:      30,
								},
								failed: map[uint64]int{22: 1},
								err:    errors.New("22: failed"),
							},
							curr: 25,
						}
					},
					2: func() workerState {
						return workerState{
							result: result{
								job: job{
									jobType: catchupJob,
									from:    11,
									to:      20,
								},
								failed: map[uint64]int{12: 1, 13: 1},
								err:    errors.Join(errors.New("12: failed"), errors.New("13: failed")),
							},
							curr: 15,
						}
					},
				},
				failed: map[uint64]retryAttempt{
					22: {count: 1},
					23: {count: 1},
					24: {count: 2},
				},
				nextJobID:   0,
				next:        31,
				networkHead: 100,
			},
			SamplingStats{
				SampledChainHead: 11,
				CatchupHead:      30,
				NetworkHead:      100,
				Failed:           map[uint64]int{22: 2, 23: 1, 24: 2, 12: 1, 13: 1},
				Workers: []WorkerStats{
					{
						JobType: recentJob,
						Curr:    25,
						From:    21,
						To:      30,
						ErrMsg:  "22: failed",
					},
					{
						JobType: catchupJob,
						Curr:    15,
						From:    11,
						To:      20,
						ErrMsg:  "12: failed\n13: failed",
					},
				},
				Concurrency: 2,
				CatchUpDone: false,
				IsRunning:   true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := tt.state.unsafeStats()
			sort.Slice(stats.Workers, func(i, j int) bool {
				return stats.Workers[i].From > stats.Workers[j].Curr
			})
			assert.Equal(t, tt.want, stats, "stats are not equal")
		})
	}
}
