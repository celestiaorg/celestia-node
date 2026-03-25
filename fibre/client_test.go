package fibre

import (
	"context"
	"errors"
	"testing"
	"time"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appfibre "github.com/celestiaorg/celestia-app/v8/fibre"
	fibretypes "github.com/celestiaorg/celestia-app/v8/x/fibre/types"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/state/txclient"
)

// mockTxClient implements txClient for testing.
type mockTxClient struct {
	uploadFn func(
		ctx context.Context,
		ns libshare.Namespace,
		data []byte,
	) (appfibre.SignedPaymentPromise, error)
	submitFn func(
		ctx context.Context,
		ns libshare.Namespace,
		data []byte,
		cfg *txclient.TxConfig,
	) (*appfibre.PutResult, *appfibre.PaymentPromise, error)
	getFn                func(ctx context.Context, ns libshare.Namespace, commitment []byte) (*appfibre.Blob, error)
	queryEscrowAccountFn func(ctx context.Context, signer string) (*fibretypes.EscrowAccount, error)
	depositFn            func(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error
	withdrawFn           func(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error
	pendingWithdrawalsFn func(ctx context.Context, signer string) ([]fibretypes.Withdrawal, error)
}

func (m *mockTxClient) Upload(
	ctx context.Context,
	ns libshare.Namespace,
	data []byte,
) (appfibre.SignedPaymentPromise, error) {
	return m.uploadFn(ctx, ns, data)
}

func (m *mockTxClient) Submit(
	ctx context.Context,
	ns libshare.Namespace,
	data []byte,
	cfg *txclient.TxConfig,
) (*appfibre.PutResult, *appfibre.PaymentPromise, error) {
	return m.submitFn(ctx, ns, data, cfg)
}

func (m *mockTxClient) Get(ctx context.Context, ns libshare.Namespace, commitment []byte) (*appfibre.Blob, error) {
	return m.getFn(ctx, ns, commitment)
}

func (m *mockTxClient) QueryEscrowAccount(ctx context.Context, signer string) (*fibretypes.EscrowAccount, error) {
	return m.queryEscrowAccountFn(ctx, signer)
}

func (m *mockTxClient) Deposit(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error {
	return m.depositFn(ctx, amount, cfg)
}

func (m *mockTxClient) Withdraw(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error {
	return m.withdrawFn(ctx, amount, cfg)
}

func (m *mockTxClient) PendingWithdrawals(ctx context.Context, signer string) ([]fibretypes.Withdrawal, error) {
	return m.pendingWithdrawalsFn(ctx, signer)
}

func testCommitment() appfibre.Commitment {
	var c appfibre.Commitment
	for i := range c {
		c[i] = byte(i)
	}
	return c
}

func testNamespace() libshare.Namespace {
	return libshare.MustNewV0Namespace([]byte("testns1234"))
}

func testPaymentPromise() *appfibre.PaymentPromise {
	return &appfibre.PaymentPromise{
		ChainID:           "test-chain",
		Namespace:         libshare.MustNewV0Namespace([]byte("testns1234")),
		UploadSize:        1024,
		BlobVersion:       0,
		Commitment:        testCommitment(),
		Height:            100,
		CreationTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Signature:         []byte("sig"),
	}
}

func TestClient_Upload(t *testing.T) {
	promise := testPaymentPromise()
	sigs := [][]byte{[]byte("sig1"), []byte("sig2")}

	mock := &mockTxClient{
		uploadFn: func(_ context.Context, _ libshare.Namespace, _ []byte) (appfibre.SignedPaymentPromise, error) {
			return appfibre.SignedPaymentPromise{
				PaymentPromise:      promise,
				ValidatorSignatures: sigs,
			}, nil
		},
	}

	client := NewClient(mock)
	result, err := client.Upload(context.Background(), promise.Namespace, []byte("data"))
	require.NoError(t, err)

	assert.Equal(t, Commitment(promise.Commitment), result.Commitment)
	assert.Equal(t, promise.ChainID, result.PaymentPromise.ChainID)
	assert.Equal(t, promise.UploadSize, result.PaymentPromise.BlobSize)
	assert.Equal(t, promise.Height, result.PaymentPromise.ValsetHeight)
	require.Len(t, result.ValidatorSignatures, 2)
	assert.Equal(t, ValidatorSignature([]byte("sig1")), result.ValidatorSignatures[0])
	assert.Equal(t, ValidatorSignature([]byte("sig2")), result.ValidatorSignatures[1])
}

func TestClient_Upload_Error(t *testing.T) {
	mock := &mockTxClient{
		uploadFn: func(_ context.Context, _ libshare.Namespace, _ []byte) (appfibre.SignedPaymentPromise, error) {
			return appfibre.SignedPaymentPromise{}, errors.New("upload failed")
		},
	}

	client := NewClient(mock)
	result, err := client.Upload(context.Background(), testNamespace(), []byte("data"))
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "upload failed")
}

func TestClient_Get(t *testing.T) {
	testData := []byte("hello fibre")
	blob, err := appfibre.NewBlob(testData, appfibre.DefaultBlobConfigV0())
	require.NoError(t, err)

	mock := &mockTxClient{
		getFn: func(_ context.Context, _ libshare.Namespace, _ []byte) (*appfibre.Blob, error) {
			return blob, nil
		},
	}

	client := NewClient(mock)
	resp, err := client.Get(context.Background(), testNamespace(), make([]byte, 32))
	require.NoError(t, err)
	assert.Equal(t, testData, resp.Data)
}

func TestClient_Get_Error(t *testing.T) {
	mock := &mockTxClient{
		getFn: func(_ context.Context, _ libshare.Namespace, _ []byte) (*appfibre.Blob, error) {
			return nil, errors.New("not found")
		},
	}

	client := NewClient(mock)
	resp, err := client.Get(context.Background(), testNamespace(), make([]byte, 32))
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestClient_Submit(t *testing.T) {
	promise := testPaymentPromise()
	commitment := testCommitment()
	blobID := appfibre.NewBlobID(0, commitment)
	sigs := [][]byte{[]byte("sig1")}

	mock := &mockTxClient{
		submitFn: func(
			_ context.Context,
			_ libshare.Namespace,
			_ []byte,
			_ *txclient.TxConfig,
		) (*appfibre.PutResult, *appfibre.PaymentPromise, error) {
			return &appfibre.PutResult{
				BlobID:              blobID,
				ValidatorSignatures: sigs,
				TxHash:              "ABCDEF123456",
				Height:              42,
			}, promise, nil
		},
	}

	client := NewClient(mock)
	result, err := client.Submit(context.Background(), promise.Namespace, []byte("data"), &txclient.TxConfig{})
	require.NoError(t, err)

	assert.Equal(t, Commitment(blobID.Commitment()), result.Commitment)
	assert.Equal(t, uint64(42), result.Height)
	assert.Equal(t, "ABCDEF123456", result.TxHash)
	assert.Equal(t, promise.ChainID, result.PaymentPromise.ChainID)
	require.Len(t, result.ValidatorSignatures, 1)
	assert.Equal(t, ValidatorSignature([]byte("sig1")), result.ValidatorSignatures[0])
}

func TestClient_Submit_Error(t *testing.T) {
	mock := &mockTxClient{
		submitFn: func(
			_ context.Context, _ libshare.Namespace, _ []byte, _ *txclient.TxConfig,
		) (*appfibre.PutResult, *appfibre.PaymentPromise, error) {
			return &appfibre.PutResult{}, nil, errors.New("tx failed")
		},
	}

	client := NewClient(mock)
	result, err := client.Submit(context.Background(), testNamespace(), []byte("data"), &txclient.TxConfig{})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClient_Account(t *testing.T) {
	mock := &mockTxClient{}
	client := NewClient(mock)
	acc := client.Account()
	require.NotNil(t, acc)
}
