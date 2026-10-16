package das

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeadLagOf(t *testing.T) {
	tests := []struct {
		name  string
		stats SamplingStats
		want  int64
	}{
		{"behind", SamplingStats{SampledChainHead: 10, NetworkHead: 100}, 90},
		{"caught up", SamplingStats{SampledChainHead: 100, NetworkHead: 100}, 0},
		{"ahead of network head", SamplingStats{SampledChainHead: 101, NetworkHead: 100}, 0},
		{"nothing sampled", SamplingStats{SampledChainHead: 0, NetworkHead: 0}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headLagOf(tt.stats))
		})
	}
}
