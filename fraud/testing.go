package fraud

import (
	"context"
	"testing"

	"github.com/ipfs/go-blockservice"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/ipld"
)

type DummyService struct {
}

func (d *DummyService) Broadcast(context.Context, Proof) error {
	return nil
}

func (d *DummyService) Subscribe(ProofType) (Subscription, error) {
	return &subscription{}, nil
}

func (d *DummyService) Get(context.Context, ProofType) ([]Proof, error) {
	return nil, nil
}

type mockStore struct {
	headers    map[int64]*header.ExtendedHeader
	headHeight int64
}

// createStore creates a mock store and adds several random
// headers.
func createStore(t *testing.T, numHeaders int) *mockStore {
	store := &mockStore{
		headers:    make(map[int64]*header.ExtendedHeader),
		headHeight: 0,
	}

	suite := header.NewTestSuite(t, numHeaders)

	for i := 0; i < numHeaders; i++ {
		header := suite.GenExtendedHeader()
		store.headers[header.Height] = header

		if header.Height > store.headHeight {
			store.headHeight = header.Height
		}
	}
	return store
}

func (m *mockStore) GetByHeight(_ context.Context, height uint64) (*header.ExtendedHeader, error) {
	return m.headers[int64(height)], nil
}

func generateByzantineError(
	ctx context.Context,
	t *testing.T,
	h *header.ExtendedHeader,
	bServ blockservice.BlockService,
) (*header.ExtendedHeader, error) {
	faultHeader := header.CreateFraudExtHeader(t, h, bServ)
	rtrv := ipld.NewRetriever(bServ)
	_, err := rtrv.Retrieve(ctx, faultHeader.DAH)
	return faultHeader, err
}
