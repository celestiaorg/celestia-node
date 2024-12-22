package blob_test

import (
	"context"
	"errors"
	"testing"

	"github.com/celestiaorg/celestia-node/blob"
	nodeblob "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/stretchr/testify/assert"
)

func TestAPI(t *testing.T) {
	// Mock API with dummy data and behaviors
	mockAPI := &nodeblob.API{
		Internal: struct {
			Submit             func(context.Context, []*blob.Blob, *blob.SubmitOptions) (uint64, error)                      `perm:"write"`
			Get                func(context.Context, uint64, libshare.Namespace, blob.Commitment) (*blob.Blob, error)        `perm:"read"`
			GetAll             func(context.Context, uint64, []libshare.Namespace) ([]*blob.Blob, error)                     `perm:"read"`
			GetProof           func(context.Context, uint64, libshare.Namespace, blob.Commitment) (*blob.Proof, error)       `perm:"read"`
			Included           func(context.Context, uint64, libshare.Namespace, *blob.Proof, blob.Commitment) (bool, error) `perm:"read"`
			GetCommitmentProof func(context.Context, uint64, libshare.Namespace, []byte) (*blob.CommitmentProof, error)      `perm:"read"`
			Subscribe          func(context.Context, libshare.Namespace) (<-chan *blob.SubscriptionResponse, error)          `perm:"read"`
		}{
			Submit: func(ctx context.Context, blobs []*blob.Blob, options *blob.SubmitOptions) (uint64, error) {
				return 42, nil
			},
			Get: func(ctx context.Context, height uint64, namespace libshare.Namespace, commitment blob.Commitment) (*blob.Blob, error) {
				return &blob.Blob{}, nil
			},
			GetAll: func(ctx context.Context, height uint64, namespaces []libshare.Namespace) ([]*blob.Blob, error) {
				return []*blob.Blob{{}, {}}, nil
			},
			GetProof: func(ctx context.Context, height uint64, namespace libshare.Namespace, commitment blob.Commitment) (*blob.Proof, error) {
				return &blob.Proof{}, nil
			},
			Included: func(ctx context.Context, height uint64, namespace libshare.Namespace, proof *blob.Proof, commitment blob.Commitment) (bool, error) {
				return true, nil
			},
			GetCommitmentProof: func(ctx context.Context, height uint64, namespace libshare.Namespace, shareCommitment []byte) (*blob.CommitmentProof, error) {
				return &blob.CommitmentProof{}, nil
			},
			Subscribe: func(ctx context.Context, namespace libshare.Namespace) (<-chan *blob.SubscriptionResponse, error) {
				ch := make(chan *blob.SubscriptionResponse, 1)
				ch <- &blob.SubscriptionResponse{}
				close(ch)
				return ch, nil
			},
		},
	}

	// Test Submit
	t.Run("Submit", func(t *testing.T) {
		height, err := mockAPI.Submit(context.Background(), []*blob.Blob{}, &blob.SubmitOptions{})
		assert.NoError(t, err)
		assert.Equal(t, uint64(42), height)
	})

	// Test Get
	t.Run("Get", func(t *testing.T) {
		blob, err := mockAPI.Get(context.Background(), 100, libshare.Namespace{}, blob.Commitment{})
		assert.NoError(t, err)
		assert.NotNil(t, blob)
	})

	// Test GetAll
	t.Run("GetAll", func(t *testing.T) {
		blobs, err := mockAPI.GetAll(context.Background(), 100, []libshare.Namespace{{}})
		assert.NoError(t, err)
		assert.Len(t, blobs, 2)
	})

	// Test GetProof
	t.Run("GetProof", func(t *testing.T) {
		proof, err := mockAPI.GetProof(context.Background(), 100, libshare.Namespace{}, blob.Commitment{})
		assert.NoError(t, err)
		assert.NotNil(t, proof)
	})

	// Test Included
	t.Run("Included", func(t *testing.T) {
		included, err := mockAPI.Included(context.Background(), 100, libshare.Namespace{}, &blob.Proof{}, blob.Commitment{})
		assert.NoError(t, err)
		assert.True(t, included)
	})

	// Test GetCommitmentProof
	t.Run("GetCommitmentProof", func(t *testing.T) {
		commitmentProof, err := mockAPI.GetCommitmentProof(context.Background(), 100, libshare.Namespace{}, []byte{})
		assert.NoError(t, err)
		assert.NotNil(t, commitmentProof)
	})

	// Test Subscribe
	t.Run("Subscribe", func(t *testing.T) {
		ch, err := mockAPI.Subscribe(context.Background(), libshare.Namespace{})
		assert.NoError(t, err)
		assert.NotNil(t, ch)

		resp := <-ch
		assert.NotNil(t, resp)
	})
}

func TestAPIErrorCases(t *testing.T) {
	// Mock API with error scenarios
	mockAPI := &nodeblob.API{
		Internal: struct {
			Submit             func(context.Context, []*blob.Blob, *blob.SubmitOptions) (uint64, error)                      `perm:"write"`
			Get                func(context.Context, uint64, libshare.Namespace, blob.Commitment) (*blob.Blob, error)        `perm:"read"`
			GetAll             func(context.Context, uint64, []libshare.Namespace) ([]*blob.Blob, error)                     `perm:"read"`
			GetProof           func(context.Context, uint64, libshare.Namespace, blob.Commitment) (*blob.Proof, error)       `perm:"read"`
			Included           func(context.Context, uint64, libshare.Namespace, *blob.Proof, blob.Commitment) (bool, error) `perm:"read"`
			GetCommitmentProof func(context.Context, uint64, libshare.Namespace, []byte) (*blob.CommitmentProof, error)      `perm:"read"`
			Subscribe          func(context.Context, libshare.Namespace) (<-chan *blob.SubscriptionResponse, error)          `perm:"read"`
		}{
			Submit: func(ctx context.Context, blobs []*blob.Blob, options *blob.SubmitOptions) (uint64, error) {
				return 0, errors.New("submit error")
			},
			Get: func(ctx context.Context, height uint64, namespace libshare.Namespace, commitment blob.Commitment) (*blob.Blob, error) {
				return nil, errors.New("get error")
			},
			GetAll: func(ctx context.Context, height uint64, namespaces []libshare.Namespace) ([]*blob.Blob, error) {
				return nil, errors.New("get all error")
			},
			GetProof: func(ctx context.Context, height uint64, namespace libshare.Namespace, commitment blob.Commitment) (*blob.Proof, error) {
				return nil, errors.New("get proof error")
			},
			Included: func(ctx context.Context, height uint64, namespace libshare.Namespace, proof *blob.Proof, commitment blob.Commitment) (bool, error) {
				return false, errors.New("included error")
			},
			GetCommitmentProof: func(ctx context.Context, height uint64, namespace libshare.Namespace, shareCommitment []byte) (*blob.CommitmentProof, error) {
				return nil, errors.New("get commitment proof error")
			},
			Subscribe: func(ctx context.Context, namespace libshare.Namespace) (<-chan *blob.SubscriptionResponse, error) {
				return nil, errors.New("subscribe error")
			},
		},
	}

	// Test Submit Error
	t.Run("Submit Error", func(t *testing.T) {
		_, err := mockAPI.Submit(context.Background(), []*blob.Blob{}, &blob.SubmitOptions{})
		assert.Error(t, err)
	})

	// Test Get Error
	t.Run("Get Error", func(t *testing.T) {
		_, err := mockAPI.Get(context.Background(), 100, libshare.Namespace{}, blob.Commitment{})
		assert.Error(t, err)
	})

	// Other error scenarios can be tested similarly...
}
