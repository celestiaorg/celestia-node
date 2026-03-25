package fibre

import (
	"context"
	"errors"
	"testing"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fibretypes "github.com/celestiaorg/celestia-app/v8/x/fibre/types"

	"github.com/celestiaorg/celestia-node/state/txclient"
)

func testCoin(amount int64) sdktypes.Coin {
	return sdktypes.NewInt64Coin("utia", amount)
}

func TestAccountClient_QueryEscrowAccount(t *testing.T) {
	mock := &mockTxClient{
		queryEscrowAccountFn: func(_ context.Context, signer string) (*fibretypes.EscrowAccount, error) {
			return &fibretypes.EscrowAccount{
				Signer:           signer,
				Balance:          testCoin(1000),
				AvailableBalance: testCoin(800),
			}, nil
		},
	}

	acc := &AccountClient{client: mock}
	result, err := acc.QueryEscrowAccount(context.Background(), "celestia1abc")
	require.NoError(t, err)

	assert.Equal(t, "celestia1abc", result.Signer)
	assert.Equal(t, testCoin(1000), result.Balance)
	assert.Equal(t, testCoin(800), result.AvailableBalance)
}

func TestAccountClient_QueryEscrowAccount_Error(t *testing.T) {
	mock := &mockTxClient{
		queryEscrowAccountFn: func(_ context.Context, _ string) (*fibretypes.EscrowAccount, error) {
			return nil, errors.New("not found")
		},
	}

	acc := &AccountClient{client: mock}
	result, err := acc.QueryEscrowAccount(context.Background(), "celestia1abc")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestAccountClient_Deposit(t *testing.T) {
	mock := &mockTxClient{
		depositFn: func(_ context.Context, _ sdktypes.Coin, _ *txclient.TxConfig) error {
			return nil
		},
	}

	acc := &AccountClient{client: mock}
	err := acc.Deposit(context.Background(), testCoin(500), &txclient.TxConfig{})
	require.NoError(t, err)
}

func TestAccountClient_Deposit_Error(t *testing.T) {
	mock := &mockTxClient{
		depositFn: func(_ context.Context, _ sdktypes.Coin, _ *txclient.TxConfig) error {
			return errors.New("insufficient funds")
		},
	}

	acc := &AccountClient{client: mock}
	err := acc.Deposit(context.Background(), testCoin(500), &txclient.TxConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient funds")
}

func TestAccountClient_Withdraw(t *testing.T) {
	mock := &mockTxClient{
		withdrawFn: func(_ context.Context, _ sdktypes.Coin, _ *txclient.TxConfig) error {
			return nil
		},
	}

	acc := &AccountClient{client: mock}
	err := acc.Withdraw(context.Background(), testCoin(200), &txclient.TxConfig{})
	require.NoError(t, err)
}

func TestAccountClient_Withdraw_Error(t *testing.T) {
	mock := &mockTxClient{
		withdrawFn: func(_ context.Context, _ sdktypes.Coin, _ *txclient.TxConfig) error {
			return errors.New("withdraw failed")
		},
	}

	acc := &AccountClient{client: mock}
	err := acc.Withdraw(context.Background(), testCoin(200), &txclient.TxConfig{})
	require.Error(t, err)
}

func TestAccountClient_PendingWithdrawals(t *testing.T) {
	mock := &mockTxClient{
		pendingWithdrawalsFn: func(_ context.Context, signer string) ([]fibretypes.Withdrawal, error) {
			return []fibretypes.Withdrawal{
				{Signer: signer, Amount: testCoin(100)},
				{Signer: signer, Amount: testCoin(200)},
			}, nil
		},
	}

	acc := &AccountClient{client: mock}
	result, err := acc.PendingWithdrawals(context.Background(), "celestia1abc")
	require.NoError(t, err)
	require.Len(t, result, 2)

	assert.Equal(t, testCoin(100), result[0].Amount)
	assert.Equal(t, testCoin(200), result[1].Amount)
	assert.Equal(t, "celestia1abc", result[0].Signer)
}

func TestAccountClient_PendingWithdrawals_Error(t *testing.T) {
	mock := &mockTxClient{
		pendingWithdrawalsFn: func(_ context.Context, _ string) ([]fibretypes.Withdrawal, error) {
			return nil, errors.New("query failed")
		},
	}

	acc := &AccountClient{client: mock}
	result, err := acc.PendingWithdrawals(context.Background(), "celestia1abc")
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestAccountClient_PendingWithdrawals_Empty(t *testing.T) {
	mock := &mockTxClient{
		pendingWithdrawalsFn: func(_ context.Context, _ string) ([]fibretypes.Withdrawal, error) {
			return []fibretypes.Withdrawal{}, nil
		},
	}

	acc := &AccountClient{client: mock}
	result, err := acc.PendingWithdrawals(context.Background(), "celestia1abc")
	require.NoError(t, err)
	assert.Empty(t, result)
}
