package headertest

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	tmrand "github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	libhead "github.com/celestiaorg/go-header"
)

func TestVerify(t *testing.T) {
	h := NewTestSuite(t, 2).GenExtendedHeaders(3)
	trusted, untrustedAdj, untrustedNonAdj := h[0], h[1], h[2]
	tests := []struct {
		prepare func() libhead.Header
		err     bool
	}{
		{
			prepare: func() libhead.Header { return untrustedAdj },
			err:     false,
		},
		{
			prepare: func() libhead.Header {
				return untrustedNonAdj
			},
			err: false,
		},
		{
			prepare: func() libhead.Header {
				untrusted := *untrustedAdj
				untrusted.ValidatorsHash = tmrand.Bytes(32)
				return &untrusted
			},
			err: true,
		},
		{
			prepare: func() libhead.Header {
				untrusted := *untrustedAdj
				untrusted.RawHeader.LastBlockID.Hash = tmrand.Bytes(32)
				return &untrusted
			},
			err: true,
		},
		{
			prepare: func() libhead.Header {
				untrustedAdj.RawHeader.Time = untrustedAdj.RawHeader.Time.Add(time.Minute)
				return untrustedAdj
			},
			err: true,
		},
		{
			prepare: func() libhead.Header {
				untrustedAdj.RawHeader.Time = untrustedAdj.RawHeader.Time.Truncate(time.Hour)
				return untrustedAdj
			},
			err: true,
		},
		{
			prepare: func() libhead.Header {
				untrustedAdj.RawHeader.ChainID = "toaster"
				return untrustedAdj
			},
			err: true,
		},
		{
			prepare: func() libhead.Header {
				untrustedAdj.RawHeader.Height++
				return untrustedAdj
			},
			err: true,
		},
		{
			prepare: func() libhead.Header {
				untrustedAdj.RawHeader.Version.App = appconsts.LatestVersion + 1
				return untrustedAdj
			},
			err: true,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			err := trusted.Verify(test.prepare())
			if test.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
