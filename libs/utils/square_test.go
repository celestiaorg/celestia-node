package utils

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/math"
)

func TestSquareSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lenShares int
		want      uint64
	}{
		{
			name:      "negative shares",
			lenShares: -1,
			want:      0,
		},
		{
			name:      "zero shares",
			lenShares: 0,
			want:      0,
		},
		{
			name:      "one share",
			lenShares: 1,
			want:      1,
		},
		{
			name:      "four shares",
			lenShares: 4,
			want:      2,
		},
		{
			name:      "nine shares",
			lenShares: 9,
			want:      3,
		},
		{
			name:      "max int shares",
			lenShares: math.MaxInt64,
			want:      3037000499,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := SquareSize(tc.lenShares)
			if got != tc.want {
				t.Errorf("SquareSize(%d) = %d, want %d", tc.lenShares, got, tc.want)
			}
		})
	}
}
