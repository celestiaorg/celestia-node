package txclient

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateFee(t *testing.T) {
	tests := []struct {
		name     string
		gasPrice float64
		gas      uint64
		wantErr  bool
	}{
		{"negative gas price", -0.5, 1000, true},
		{"NaN gas price", math.NaN(), 1000, true},
		{"+Inf gas price", math.Inf(1), 1000, true},
		{"-Inf gas price", math.Inf(-1), 1000, true},
		{"fee overflows int64", 1e18, 1_000_000, true},
		{"zero gas price is valid", 0, 1000, false},
		{"typical gas price is valid", 0.002, 100_000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFee(tt.gasPrice, tt.gas)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
