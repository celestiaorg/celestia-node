package full

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestFullWindowConst exists to ensure that any changes to the sampling window
// are deliberate.
func TestFullWindowConst(t *testing.T) {
	assert.Equal(t, time.Duration(Window), (30*24*time.Hour)+time.Hour)
}
