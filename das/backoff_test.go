package das

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_exponentialBackoff(t *testing.T) {
	type args struct {
		baseInterval time.Duration
		factor       int
		amount       int
	}
	tests := []struct {
		name string
		args args
		want []time.Duration
	}{
		{
			name: "defaults",
			args: args{
				baseInterval: time.Minute,
				factor:       4,
				amount:       4,
			},
			want: []time.Duration{
				time.Minute,
				4 * time.Minute,
				16 * time.Minute,
				64 * time.Minute,
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t,
				tt.want, exponentialBackoff(tt.args.baseInterval, tt.args.factor, tt.args.amount),
				"exponentialBackoff(%v, %v, %v)", tt.args.baseInterval, tt.args.factor, tt.args.amount)
		})
	}
}

func Test_retryStrategy_nextRetry(t *testing.T) {
	tNow := time.Now()
	type args struct {
		tryCount    int
		lastAttempt time.Time
	}
	tests := []struct {
		name                string
		strategy            retryStrategy
		args                args
		wantRetry           retry
		wantRetriesExceeded bool
	}{
		{
			name:     "empty_stratage",
			strategy: newRetryStrategy(nil),
			args: args{
				tryCount:    1,
				lastAttempt: tNow,
			},
			wantRetry: retry{
				count: 1,
			},
			wantRetriesExceeded: false,
		},
		{
			name:     "before_limit",
			strategy: newRetryStrategy([]time.Duration{time.Second, time.Minute}),
			args: args{
				tryCount:    2,
				lastAttempt: tNow,
			},
			wantRetry: retry{
				count: 2,
				after: tNow.Add(time.Minute),
			},
			wantRetriesExceeded: false,
		},
		{
			name:     "after_limit",
			strategy: newRetryStrategy([]time.Duration{time.Second, time.Minute}),
			args: args{
				tryCount:    3,
				lastAttempt: tNow,
			},
			wantRetry: retry{
				count: 3,
				after: tNow.Add(time.Minute),
			},
			wantRetriesExceeded: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := retryStrategy{
				backoffIntervals: tt.strategy.backoffIntervals,
			}
			gotRetry, gotRetriesExceeded := s.nextRetry(tt.args.tryCount, tt.args.lastAttempt)
			assert.Equalf(t, tt.wantRetry, gotRetry,
				"nextRetry(%v, %v)", tt.args.tryCount, tt.args.lastAttempt)
			assert.Equalf(t, tt.wantRetriesExceeded, gotRetriesExceeded,
				"nextRetry(%v, %v)", tt.args.tryCount, tt.args.lastAttempt)
		})
	}
}
