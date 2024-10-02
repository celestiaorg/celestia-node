package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_ctxWithSplitTimeout(t *testing.T) {
	type args struct {
		ctxTimeout  time.Duration
		splitFactor []int
		minTimeout  time.Duration
	}
	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{
			name: "ctxTimeout > minTimeout, splitFactor <= 0",
			args: args{
				ctxTimeout:  3 * time.Minute,
				splitFactor: []int{-1, 0},
				minTimeout:  time.Minute,
			},
			want: time.Minute,
		},
		{
			name: "ctxTimeout > minTimeout, splitFactor = 1",
			args: args{
				ctxTimeout:  3 * time.Minute,
				splitFactor: []int{1},
				minTimeout:  time.Minute,
			},
			want: 3 * time.Minute,
		},
		{
			name: "ctxTimeout > minTimeout, splitFactor = 2",
			args: args{
				ctxTimeout:  3 * time.Minute,
				splitFactor: []int{2},
				minTimeout:  time.Minute,
			},
			want: 3 * time.Minute / 2,
		},
		{
			name: "ctxTimeout > minTimeout, resulted timeout limited by minTimeout",
			args: args{
				ctxTimeout:  3 * time.Minute,
				splitFactor: []int{3, 4, 5},
				minTimeout:  time.Minute,
			},
			want: time.Minute,
		},
		{
			name: "ctxTimeout < minTimeout",
			args: args{
				ctxTimeout:  time.Minute,
				splitFactor: []int{-1, 0, 1, 2, 3},
				minTimeout:  2 * time.Minute,
			},
			want: time.Minute,
		},
		{
			name: "minTimeout = 0, splitFactor <= 1",
			args: args{
				ctxTimeout:  time.Minute,
				splitFactor: []int{-1, 0, 1},
				minTimeout:  0,
			},
			want: time.Minute,
		},
		{
			name: "minTimeout = 0, splitFactor > 1",
			args: args{
				ctxTimeout:  time.Minute,
				splitFactor: []int{2},
				minTimeout:  0,
			},
			want: time.Minute / 2,
		},
		{
			name: "no context timeout",
			args: args{
				ctxTimeout:  0,
				splitFactor: []int{-1, 0, 1, 2},
				minTimeout:  time.Minute,
			},
			want: time.Minute,
		},
		{
			name: "no context timeout, minTimeout = 0",
			args: args{
				ctxTimeout:  0,
				splitFactor: []int{-1, 0, 1, 2},
				minTimeout:  0,
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, sf := range tt.args.splitFactor {
				ctx, cancel := context.WithCancel(context.Background())
				// add timeout if original context should have it
				if tt.args.ctxTimeout > 0 {
					ctx, cancel = context.WithTimeout(ctx, tt.args.ctxTimeout)
				}
				t.Cleanup(cancel)
				got, _ := CtxWithSplitTimeout(ctx, sf, tt.args.minTimeout)
				dl, ok := got.Deadline()
				// in case no deadline is found in ctx or not expected to be found, check both cases apply at the
				// same time
				if !ok || tt.want == 0 {
					require.False(t, ok)
					require.Equal(t, tt.want, time.Duration(0))
					continue
				}
				d := time.Until(dl)
				require.True(t, d <= tt.want+time.Second)
				require.True(t, d >= tt.want-time.Second)
			}
		})
	}
}
