package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIndex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr string
	}{
		{name: "zero", input: "0", want: 0},
		{name: "positive", input: "42", want: 42},
		{name: "negative rejected", input: "-1", wantErr: "must be non-negative"},
		{name: "non-numeric rejected", input: "abc", wantErr: "invalid syntax"},
		{name: "overflow rejected", input: "9223372036854775808", wantErr: "value out of range"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseIndex(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
