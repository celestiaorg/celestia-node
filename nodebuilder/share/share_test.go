package share

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/share/mocks"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

func Test_EmptyCARExists(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := eds.NewStore(tmpDir, ds)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds := share.EmptyExtendedDataSquare()
	dah := da.NewDataAvailabilityHeader(eds)

	// add empty EDS to store
	err = ensureEmptyCARExists(ctx, edsStore)
	assert.NoError(t, err)

	// assert that the empty car exists
	has, err := edsStore.Has(ctx, dah.Hash())
	assert.True(t, has)
	assert.NoError(t, err)

	// assert that the empty car is, in fact, empty
	emptyEds, err := edsStore.Get(ctx, dah.Hash())
	assert.Equal(t, eds.Flattened(), emptyEds.Flattened())
	assert.NoError(t, err)
}

// fnSpyStatus is a structure to store
// the function spying results
type fnSpyStatus struct {
	called bool
	times  uint
}

func (fss *fnSpyStatus) Called() bool {
	return fss.called
}

func (fss *fnSpyStatus) Times(i uint) bool {
	return fss.times == i
}

func newFnSpyStatus() *fnSpyStatus {
	return &fnSpyStatus{
		called: false,
		times:  0,
	}
}

// A spy getter that will spy on
// the methods from the share.Getter interface
type spyGetter struct {
	getShare             *fnSpyStatus
	getEDS               *fnSpyStatus
	getSharesByNamespace *fnSpyStatus

	// embed the Getter interface
	share.Getter
}

func (sg *spyGetter) GetShare(ctx context.Context, root *share.Root, row, col int) (share.Share, error) {
	sg.getShare.called = true
	sg.getShare.times++
	return sg.Getter.GetShare(ctx, root, row, col)
}

func (sg *spyGetter) GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	sg.getEDS.called = true
	sg.getEDS.times++
	return sg.Getter.GetEDS(ctx, root)
}

func (sg *spyGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	id namespace.ID,
) (share.NamespacedShares, error) {
	sg.getSharesByNamespace.called = true
	sg.getSharesByNamespace.times++
	return sg.Getter.GetSharesByNamespace(ctx, root, id)
}

func newSpyGetter(getter share.Getter) *spyGetter {
	getShare := newFnSpyStatus()
	getEDS := newFnSpyStatus()
	getSharesByNamespace := newFnSpyStatus()
	return &spyGetter{getShare, getEDS, getSharesByNamespace, getter}
}

func Test_InstrumentedShareGetter(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockGetter := mocks.NewMockGetter(mockCtrl)

	mod := newModule(mockGetter, nil)
	proxiedGetter, err := newInstrument(mockGetter)
	assert.NoError(t, err)

	proxiedGetterSpy := newSpyGetter(proxiedGetter)

	smod, ok := mod.(*module)
	assert.True(t, ok)

	smod.Getter = proxiedGetterSpy

	// prepare the arguments
	eds := share.EmptyExtendedDataSquare()
	root := da.NewDataAvailabilityHeader(eds)
	ctx := context.Background()

	mockGetter.EXPECT().GetShare(ctx, &root, 1, 1).Times(1)

	// perform the call
	_, err = mod.GetShare(ctx, &root, 1, 1)
	assert.NoError(t, err)

	assert.True(t, proxiedGetterSpy.getShare.Called())
	assert.True(t, proxiedGetterSpy.getShare.Times(1))
}
