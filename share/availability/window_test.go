package availability

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConsts exists to ensure that any changes to the sampling windows
// are deliberate.
func TestConsts(t *testing.T) {
	assert.Equal(t, SamplingWindow, 7*24*time.Hour)
	assert.Equal(t, RequestWindow, 7*24*time.Hour)
	assert.Equal(t, StorageWindow, RequestWindow+time.Hour)
}
