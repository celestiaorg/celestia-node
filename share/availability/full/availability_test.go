package full

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"

	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/mocks"
)

func TestShareAvailableOverMocknet_Full(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net := availability_test.NewTestDAGNet(ctx, t)
	_, root := RandNode(net, 32)
	nd := Node(net)
	net.ConnectAll()

	err := nd.SharesAvailable(ctx, root)
	assert.NoError(t, err)
}

func TestSharesAvailable_Full(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// RandServiceWithSquare creates a NewShareAvailability inside, so we can test it
	getter, dah := GetterWithRandSquare(t, 16)
	avail := TestAvailability(t, getter)
	err := avail.SharesAvailable(ctx, dah)
	assert.NoError(t, err)
}

func TestSharesAvailable_StoresToEDSStore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// RandServiceWithSquare creates a NewShareAvailability inside, so we can test it
	getter, dah := GetterWithRandSquare(t, 16)
	avail := TestAvailability(t, getter)
	err := avail.SharesAvailable(ctx, dah)
	assert.NoError(t, err)

	has, err := avail.store.Has(ctx, dah.Hash())
	assert.NoError(t, err)
	assert.True(t, has)
}

func TestSharesAvailable_Full_ErrNotAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	getter := mocks.NewMockGetter(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eds := edstest.RandEDS(t, 4)
	dah, err := da.NewDataAvailabilityHeader(eds)
	require.NoError(t, err)
	avail := TestAvailability(t, getter)

	errors := []error{share.ErrNotFound, context.DeadlineExceeded}
	for _, getterErr := range errors {
		getter.EXPECT().GetEDS(gomock.Any(), gomock.Any()).Return(nil, getterErr)
		err := avail.SharesAvailable(ctx, &dah)
		require.ErrorIs(t, err, share.ErrNotAvailable)
	}
}
