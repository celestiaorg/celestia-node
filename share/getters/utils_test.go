package getters

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ErrorContains(t *testing.T) {
	err1 := errors.New("1")
	err2 := errors.New("2")

	w1 := func(err error) error {
		return fmt.Errorf("wrap1: %w", err)
	}
	w2 := func(err error) error {
		return fmt.Errorf("wrap1: %w", err)
	}

	type args struct {
		err    error
		target error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"nil err",
			args{
				err:    nil,
				target: err1,
			},
			false,
		},
		{
			"nil target",
			args{
				err:    err1,
				target: nil,
			},
			true,
		},
		{
			"errors.Is true",
			args{
				err:    w1(err1),
				target: err1,
			},
			true,
		},
		{
			"errors.Is false",
			args{
				err:    w1(err1),
				target: err2,
			},
			false,
		},
		{
			"same wrap but different base error",
			args{
				err:    w1(err1),
				target: w1(err2),
			},
			false,
		},
		{
			"both wrapped true",
			args{
				err:    w1(err1),
				target: w2(err1),
			},
			true,
		},
		{
			"both wrapped false",
			args{
				err:    w1(err1),
				target: w2(err2),
			},
			false,
		},
		{
			"multierr first in slice",
			args{
				err:    errors.Join(w1(err1), w2(err2)),
				target: w2(err1),
			},
			true,
		},
		{
			"multierr second in slice",
			args{
				err:    errors.Join(w1(err1), w2(err2)),
				target: w1(err2),
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t,
				tt.want,
				ErrorContains(tt.args.err, tt.args.target),
				"ErrorContains(%v, %v)", tt.args.err, tt.args.target)
		})
	}
}

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
				got, _ := ctxWithSplitTimeout(ctx, sf, tt.args.minTimeout)
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
