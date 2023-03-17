package sync

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/libs/header/test"
)

func TestAddParallel(t *testing.T) {
	var pending ranges[*test.DummyHeader]

	n := 500
	suite := test.NewTestSuite(t)
	headers := suite.GenDummyHeaders(n)

	wg := &sync.WaitGroup{}
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			pending.Add(headers[i])
			wg.Done()
		}(i)
	}
	wg.Wait()

	last := uint64(0)
	for _, r := range pending.ranges {
		assert.Greater(t, r.start, last)
		last = r.start
	}
}

func TestRangeGet(t *testing.T) {
	n := 300
	suite := test.NewTestSuite(t)
	headers := suite.GenDummyHeaders(n)

	r := newRange(headers[200])
	r.Append(headers[201:]...)

	truncated := r.Get(100)
	assert.Len(t, truncated, 0)
}
