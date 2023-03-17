package sync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/libs/header/mocks"
	"github.com/celestiaorg/celestia-node/libs/header/test"
)

func TestSyncStore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ts := test.NewTestSuite(t)
	s := mocks.NewStore[*test.DummyHeader](t, ts, 100)
	ss := syncStore[*test.DummyHeader]{Store: s}

	h, err := ss.Head(ctx)
	assert.NoError(t, err)
	assert.Equal(t, ts.Head(), h)

	err = ss.Append(ctx, ts.GetRandomHeader())
	assert.NoError(t, err)

	h, err = ss.Head(ctx)
	assert.NoError(t, err)
	assert.Equal(t, ts.Head(), h)
}
