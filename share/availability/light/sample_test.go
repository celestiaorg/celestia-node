package light

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSampleSquare(t *testing.T) {
	tests := []struct {
		width   int
		samples int
	}{
		{width: 10, samples: 5},
		{width: 500, samples: 90},
	}

	for _, tt := range tests {
		ss, err := SampleSquare(tt.width, tt.samples)
		assert.NoError(t, err)
		assert.Len(t, ss, tt.samples)
		// check points are within width
		for _, s := range ss {
			assert.Less(t, s.Row, tt.width)
			assert.Less(t, s.Col, tt.width)
		}
		// checks samples are not equal
		for i, s1 := range ss {
			for j, s2 := range ss {
				if i != j {
					assert.NotEqualValues(t, s1, s2)
				}
			}
		}
	}
}
