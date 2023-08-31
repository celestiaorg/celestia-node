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
