package canary

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

type headerFixture struct {
	headers []*header.ExtendedHeader
	hashes  int
	upper   uint64
	missing bool
}

func (f *headerFixture) NetworkHead(context.Context) (*header.ExtendedHeader, error) {
	return f.headers[len(f.headers)-1], nil
}

func (f *headerFixture) LocalHead(context.Context) (*header.ExtendedHeader, error) {
	return f.headers[len(f.headers)-1], nil
}

func (f *headerFixture) Tail(context.Context) (*header.ExtendedHeader, error) {
	return f.headers[0], nil
}

func (f *headerFixture) GetByHeight(_ context.Context, h uint64) (*header.ExtendedHeader, error) {
	for _, x := range f.headers {
		if x.Height() == h {
			return x, nil
		}
	}
	return nil, libhead.ErrNotFound
}

func (f *headerFixture) GetByHash(_ context.Context, h libhead.Hash) (*header.ExtendedHeader, error) {
	f.hashes++
	if !f.missing {
		for _, x := range f.headers {
			if x.Hash().String() == h.String() {
				return x, nil
			}
		}
	}
	return nil, libhead.ErrNotFound
}

func (f *headerFixture) GetRangeByHeight(
	_ context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	f.upper = to
	if to-from.Height()-1 > libhead.MaxRangeRequestSize {
		return nil, libhead.ErrHeadersLimitExceeded
	}
	var out []*header.ExtendedHeader
	for _, h := range f.headers {
		if h.Height() > from.Height() && h.Height() < to {
			out = append(out, h)
		}
	}
	return out, nil
}

func TestLocalRangeRejectsInvalidCountBeforeRPC(t *testing.T) {
	suite := headertest.NewTestSuite(t)
	f := &headerFixture{headers: suite.GenExtendedHeaders(2)}
	for _, n := range []int{0, -1, int(10001)} {
		_, err := LocalSuccessors(context.Background(), f, f.headers[0], n, "test")
		require.Error(t, err)
		require.Zero(t, f.hashes)
	}
}

func TestHeadTailWindowUsesActualTimestamps(t *testing.T) {
	now := time.Now().UTC()
	suite := headertest.NewTestSuite(
		t,
		headertest.WithStartTime(now.Add(-time.Hour)),
		headertest.WithBlockTime(time.Minute),
	)
	hs := suite.GenExtendedHeaders(61)
	require.NoError(t, ValidateHeadTail(hs[60], hs[0], "test", now, time.Hour, time.Minute, time.Second))
	require.Error(t, ValidateHeadTail(hs[60], hs[0], "test", now, time.Hour-time.Nanosecond, time.Minute, time.Second))
	require.Error(t, ValidateHeadTail(hs[60], hs[0], "wrong", now, time.Hour, time.Minute, time.Second))
	require.Error(t, ValidateHeadTail(hs[0], hs[60], "test", now, time.Hour, time.Minute, time.Second))
	require.Error(
		t,
		ValidateHeadTail(hs[60], hs[0], "test", now.Add(-2*time.Second), time.Hour, time.Minute, time.Second),
	)
	require.Error(
		t,
		ValidateHeadTail(hs[60], hs[0], "test", now.Add(2*time.Minute), time.Hour, time.Minute, time.Second),
	)
	require.Error(t, ValidateHeadTail(hs[60], nil, "test", now, time.Hour, time.Minute, time.Second))
}

func TestHeadTailFailureRecordsTailAge(t *testing.T) {
	now := time.Now().UTC()
	suite := headertest.NewTestSuite(
		t,
		headertest.WithStartTime(now.Add(-time.Hour)),
		headertest.WithBlockTime(time.Minute),
	)
	hs := suite.GenExtendedHeaders(61)
	window := time.Hour - time.Minute
	err := ValidateHeadTail(hs[60], hs[0], "test", now, window, time.Minute, time.Second)
	require.EqualError(t, err, "tail outside storage window")

	ev := headTailEvidence(err, hs[60], hs[0], int64(window/time.Second))
	require.Equal(t, "tail outside storage window", ev[evidenceReason])
	require.Equal(t, hs[60].Height(), ev["head_height"])
	require.Equal(t, hs[0].Height(), ev["tail_height"])
	require.InDelta(t, time.Hour.Seconds(), ev["tail_age_seconds"], 0.001)
	require.EqualValues(t, 3540, ev["storage_window_seconds"])

	incomplete := headTailEvidence(errors.New("incomplete native header"), hs[60], nil, 3540)
	require.NotContains(t, incomplete, "tail_age_seconds")
}

func TestLocalSuccessorsUsesNativeRangeAndEveryStoredHash(t *testing.T) {
	suite := headertest.NewTestSuite(
		t,
		headertest.WithStartTime(time.Now().Add(-time.Hour)),
		headertest.WithBlockTime(time.Second),
	)
	f := &headerFixture{headers: suite.GenExtendedHeaders(101)}
	got, err := LocalSuccessors(context.Background(), f, f.headers[0], 100, "test")
	require.NoError(t, err)
	require.Len(t, got, 100)
	require.Equal(t, uint64(102), f.upper)
	require.Equal(t, 101, f.hashes)
	f.missing = true
	_, err = LocalSuccessors(context.Background(), f, f.headers[0], 100, "test")
	require.Error(t, err, "network-visible headers are not local storage")
}
