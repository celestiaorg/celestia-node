package light

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestLightWindowConst exists to ensure that any changes to the sampling window
// are deliberate.
func TestLightWindowConst(t *testing.T) {
	assert.Equal(t, time.Duration(Window), 30*24*time.Hour)
}
