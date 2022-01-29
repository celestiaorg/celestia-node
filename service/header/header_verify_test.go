package header

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	tmrand "github.com/tendermint/tendermint/libs/rand"
)

func TestVerifyAdjacent(t *testing.T) {
	h := NewTestSuite(t, 2).GenExtendedHeaders(2)
	trusted, untrusted := h[0], h[1]
	tests := []struct {
		prepare func()
		err     bool
	}{
		{
			prepare: func() {},
			err:     false,
		},
		{
			prepare: func() {
				untrusted.ValidatorsHash = tmrand.Bytes(32)
			},
			err: true,
		},
		{
			prepare: func() {
				untrusted.Time = untrusted.Time.Add(time.Minute)
			},
			err: true,
		},
		{
			prepare: func() {
				untrusted.Time = untrusted.Time.Truncate(time.Hour)
			},
			err: true,
		},
		{
			prepare: func() {
				untrusted.ChainID = "toaster"
			},
			err: true,
		},
		{
			prepare: func() {
				untrusted.Height++
			},
			err: true,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			test.prepare()
			err := trusted.VerifyAdjacent(untrusted)
			if test.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
