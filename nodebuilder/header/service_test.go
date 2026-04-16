package header

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
)

func TestGetByHeightHandlesError(t *testing.T) {
	serv := Service{
		syncer: &errorSyncer[*header.ExtendedHeader]{},
	}

	assert.NotPanics(t, func() {
		h, err := serv.GetByHeight(context.Background(), 100)
		assert.Error(t, err)
		assert.Nil(t, h)
	})
}

type errorSyncer[H libhead.Header[H]] struct{}

func (d *errorSyncer[H]) Head(context.Context, ...libhead.HeadOption[H]) (H, error) {
	var zero H
	return zero, fmt.Errorf("dummy error")
}

func (d *errorSyncer[H]) State() sync.State {
	return sync.State{}
}

func (d *errorSyncer[H]) SyncWait(context.Context) error {
	return fmt.Errorf("dummy error")
}

func TestGetRangeByHeight_MaxRangeRequestSize(t *testing.T) {
	from := &header.ExtendedHeader{
		RawHeader: header.RawHeader{Height: 100},
	}

	serv := Service{}

	t.Run("rejects range exceeding MaxRangeRequestSize", func(t *testing.T) {
		// request 65 headers: from.Height()+1=101 to 166 exclusive = 65 headers
		to := from.Height() + 1 + libhead.MaxRangeRequestSize + 1
		_, err := serv.GetRangeByHeight(context.Background(), from, to)
		assert.ErrorContains(t, err, "MaxRangeRequestSize")
	})

	t.Run("accepts range at MaxRangeRequestSize", func(t *testing.T) {
		// request exactly 64 headers: from.Height()+1=101 to 165 exclusive = 64 headers
		// this will pass the guard but panic on nil store, proving the guard accepted it.
		to := from.Height() + 1 + libhead.MaxRangeRequestSize
		assert.Panics(t, func() {
			serv.GetRangeByHeight(context.Background(), from, to) //nolint:errcheck
		})
	})
}
