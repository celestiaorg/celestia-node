package das

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

// TestDeriveSampleTimeout tests that every legal extended square width derives its
// pinned timeout.
func TestDeriveSampleTimeout(t *testing.T) {
	tests := []struct {
		edsWidth int
		expected time.Duration
	}{
		{edsWidth: 2, expected: 20*time.Second + 1953125*time.Nanosecond},
		{edsWidth: 4, expected: 20*time.Second + 7812500*time.Nanosecond},
		{edsWidth: 8, expected: 20*time.Second + 31250*time.Microsecond},
		{edsWidth: 16, expected: 20*time.Second + 125*time.Millisecond},
		{edsWidth: 32, expected: 20*time.Second + 500*time.Millisecond},
		{edsWidth: 64, expected: 22 * time.Second},
		{edsWidth: 128, expected: 28 * time.Second},
		{edsWidth: 256, expected: 52 * time.Second},
		{edsWidth: 512, expected: 148 * time.Second},
		{edsWidth: 1024, expected: 532 * time.Second},
	}

	// the table must cover up to the widest legal square
	require.Equal(t, maxEDSWidth, tests[len(tests)-1].edsWidth)

	for _, tt := range tests {
		require.Equal(t, tt.expected, deriveSampleTimeout(tt.edsWidth), "width %d", tt.edsWidth)
	}
}

// TestDeriveSampleTimeoutIllegalWidth tests that a width outside the legal range returns
// the widest legal square's timeout.
func TestDeriveSampleTimeoutIllegalWidth(t *testing.T) {
	ceiling := deriveSampleTimeout(maxEDSWidth)

	for _, edsWidth := range []int{0, -1, -maxEDSWidth, maxEDSWidth + 1, 1 << 40} {
		require.Equal(t, ceiling, deriveSampleTimeout(edsWidth), "width %d", edsWidth)
	}
}

// TestWorkerSampleTimeout tests that a worker derives its deadline from the header's
// square size when SampleTimeout is zero, and honors a configured value as an override.
func TestWorkerSampleTimeout(t *testing.T) {
	const odsWidth = 4

	eds := edstest.RandEDS(t, odsWidth)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	// DAH roots span the extended square
	require.Equal(t, 2*odsWidth, len(eh.DAH.RowRoots))

	tests := []struct {
		name       string
		configured time.Duration
		expected   time.Duration
	}{
		{name: "derived when unset", configured: 0, expected: deriveSampleTimeout(2 * odsWidth)},
		{name: "configured value overrides", configured: 5 * time.Second, expected: 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deadline time.Duration
			sample := func(ctx context.Context, _ *header.ExtendedHeader) error {
				dl, ok := ctx.Deadline()
				require.True(t, ok)
				deadline = time.Until(dl)
				return nil
			}

			// header on the job means the nil getter is never used
			j := job{jobType: recentJob, from: 1, to: 1, header: eh}
			w := newWorker(j, nil, sample, nil)

			require.NoError(t, w.sample(context.Background(), tt.configured, 1))
			require.InDelta(t, float64(tt.expected), float64(deadline), float64(time.Second))
		})
	}
}
