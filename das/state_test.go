package das

import (
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func Test_coordinatorStateReset(t *testing.T) {
	t.Run("moves next and clears failed/inRetry", func(t *testing.T) {
		s := newCoordinatorState(DefaultParameters())
		s.next = 100
		s.networkHead = 200
		s.failed = map[uint64]retryAttempt{50: {count: 1}, 150: {count: 2}}
		s.inRetry = map[uint64]retryAttempt{60: {count: 1}}

		s.reset(70)

		assert.Equal(t, uint64(70), s.next, "next should move to the reset height")
		assert.Empty(t, s.failed, "failed heights should be cleared")
		assert.Empty(t, s.inRetry, "in-retry heights should be cleared")
	})

	t.Run("re-arms catch-up after it was done", func(t *testing.T) {
		s := newCoordinatorState(DefaultParameters())
		s.next = 201
		s.networkHead = 200
		// simulate catch-up completed
		s.checkDone()
		assert.True(t, s.catchUpDone.Load(), "catch-up should be done before reset")

		// reset to a height below the head re-opens the catch-up range
		s.reset(100)

		assert.Equal(t, uint64(100), s.next)
		assert.False(t, s.catchUpDone.Load(), "catch-up should be re-armed after reset below head")
	})
}
