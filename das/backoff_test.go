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
			},
		},
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
		retry       retryAttempt
		lastAttempt time.Time
	}
	tests := []struct {
		name                string
		backoff             retryStrategy
		args                args
		wantRetry           retryAttempt
		wantMaxDelay        time.Duration
		wantRetriesExceeded bool
	}{
		{
			name:    "empty_strategy",
			backoff: newRetryStrategy(nil),
			args: args{
				retry:       retryAttempt{count: 1},
				lastAttempt: tNow,
			},
			wantRetry: retryAttempt{
				count: 2,
			},
			wantRetriesExceeded: false,
		},
		{
			name:    "before_limit",
			backoff: newRetryStrategy([]time.Duration{time.Second, time.Minute}),
			args: args{
				retry:       retryAttempt{count: 1},
				lastAttempt: tNow,
			},
			wantRetry: retryAttempt{
				count: 2,
			},
			wantMaxDelay:        time.Minute,
			wantRetriesExceeded: false,
		},
		{
			name:    "after_limit",
			backoff: newRetryStrategy([]time.Duration{time.Second, time.Minute}),
			args: args{
				retry:       retryAttempt{count: 2},
				lastAttempt: tNow,
			},
			wantRetry: retryAttempt{
				count: 3,
			},
			wantMaxDelay:        time.Minute,
			wantRetriesExceeded: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRetry, gotRetriesExceeded := tt.backoff.nextRetry(tt.args.retry, tt.args.lastAttempt)
			assert.Equalf(t, tt.wantRetry.count, gotRetry.count,
				"nextRetry(%v, %v)", tt.args.retry, tt.args.lastAttempt)
			assert.Equalf(t, tt.wantRetriesExceeded, gotRetriesExceeded,
				"nextRetry(%v, %v)", tt.args.retry, tt.args.lastAttempt)
			if tt.wantMaxDelay == 0 {
				assert.True(t, gotRetry.after.IsZero())
				return
			}
			assert.False(t, gotRetry.after.Before(tt.args.lastAttempt))
			assert.True(t, gotRetry.after.Before(tt.args.lastAttempt.Add(tt.wantMaxDelay)))
		})
	}
}
