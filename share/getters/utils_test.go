package getters

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
		{"nil err",
			args{
				err:    nil,
				target: err1,
			},
			false,
		},
		{"nil target",
			args{
				err:    err1,
				target: nil,
			},
			true,
		},
		{"errors.Is true",
			args{
				err:    w1(err1),
				target: err1,
			},
			true,
		},
		{"errors.Is false",
			args{
				err:    w1(err1),
				target: err2,
			},
			false,
		},
		{"same wrap but different base error",
			args{
				err:    w1(err1),
				target: w1(err2),
			},
			false,
		},
		{"both wrapped true",
			args{
				err:    w1(err1),
				target: w2(err1),
			},
			true,
		},
		{"both wrapped false",
			args{
				err:    w1(err1),
				target: w2(err2),
			},
			false,
		},
		{"multierr first in slice",
			args{
				err:    errors.Join(w1(err1), w2(err2)),
				target: w2(err1),
			},
			true,
		},
		{"multierr second in slice",
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
