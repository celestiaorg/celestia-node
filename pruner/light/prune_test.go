package light

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/header/headertest"
)

func TestIsWithinAvailabilityWindow(t *testing.T) {
	var tests = []struct {
		toSubtract   time.Duration
		withinWindow bool
	}{
		{toSubtract: time.Hour, withinWindow: true},
		{toSubtract: 0, withinWindow: true},
		{toSubtract: time.Hour * 24 * 29, withinWindow: true},
		{toSubtract: time.Hour * 24 * 365, withinWindow: false},
		{toSubtract: (time.Hour * 24 * 30) - time.Minute, withinWindow: true},
	}

	p := NewPruner()

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			eh := headertest.RandExtendedHeader(t)
			eh.RawHeader.Time = eh.RawHeader.Time.Add(-tt.toSubtract)

			assert.Equal(t, tt.withinWindow, p.IsWithinAvailabilityWindow(eh, time.Hour*24*30))
		})
	}
}
