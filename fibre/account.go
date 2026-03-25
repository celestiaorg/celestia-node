package fibre

import (
	"context"
	"time"

	sdktypes "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-node/state/txclient"
)

// EscrowAccount represents a Fibre escrow account used for prepaid blob storage fees.
type EscrowAccount struct {
	// Signer is the account address that owns the escrow.
	Signer string `json:"signer"`
	// Balance is the total deposited balance in the escrow account.
	Balance sdktypes.Coin `json:"balance"`
	// AvailableBalance is the balance available for spending (excluding pending withdrawals).
	AvailableBalance sdktypes.Coin `json:"available_balance"`
}

// PendingWithdrawal represents a withdrawal request from a Fibre escrow account
// that is pending the unbonding period.
type PendingWithdrawal struct {
	// Signer is the account address that requested the withdrawal.
	Signer string `json:"signer"`
	// Amount is the withdrawal amount.
	Amount sdktypes.Coin `json:"amount"`
	// RequestedTimestamp is when the withdrawal was requested.
	RequestedTimestamp time.Time `json:"requested_timestamp"`
	// AvailableTimestamp is when the withdrawal will become available for claim.
	AvailableTimestamp time.Time `json:"available_timestamp"`
}

// AccountClient provides access to Fibre escrow account operations.
type AccountClient struct {
	client  client
	metrics *accountMetrics
}

func (a *AccountClient) QueryEscrowAccount(ctx context.Context, signer string) (_ *EscrowAccount, err error) {
	if a == nil {
		return nil, ErrClientNotAvailable
	}

	start := time.Now()
	defer func() {
		a.metrics.observeQuery(ctx, time.Since(start), err)
	}()

	log.Infow("querying escrow account", "signer", signer)

	account, err := a.client.QueryEscrowAccount(ctx, signer)
	if err != nil {
		log.Errorw("querying escrow account", "err", err, "signer", signer)
		return nil, err
	}

	log.Debugw("escrow account found",
		"signer", signer,
		"balance", account.Balance,
		"available-balance", account.AvailableBalance,
	)
	return &EscrowAccount{
		Signer:           account.Signer,
		Balance:          account.Balance,
		AvailableBalance: account.AvailableBalance,
	}, nil
}

func (a *AccountClient) Deposit(
	ctx context.Context,
	amount sdktypes.Coin,
	cfg *txclient.TxConfig,
) (err error) {
	if a == nil {
		return ErrClientNotAvailable
	}

	start := time.Now()
	defer func() {
		a.metrics.observeDeposit(ctx, time.Since(start), amount.Denom, err)
	}()

	log.Infow("depositing to escrow", "amount", amount)
	if err := a.client.Deposit(ctx, amount, cfg); err != nil {
		log.Errorw("depositing to escrow", "err", err, "amount", amount)
		return err
	}

	log.Debugw("deposit successful", "amount", amount)
	return nil
}

func (a *AccountClient) Withdraw(
	ctx context.Context,
	amount sdktypes.Coin,
	cfg *txclient.TxConfig,
) (err error) {
	if a == nil {
		return ErrClientNotAvailable
	}

	start := time.Now()
	defer func() {
		a.metrics.observeWithdraw(ctx, time.Since(start), amount.Denom, err)
	}()

	log.Infow("requesting withdrawal", "amount", amount)
	if err := a.client.Withdraw(ctx, amount, cfg); err != nil {
		log.Errorw("requesting withdrawal", "err", err, "amount", amount)
		return err
	}

	log.Debugw("withdrawal requested", "amount", amount)
	return nil
}

func (a *AccountClient) PendingWithdrawals(ctx context.Context, signer string) (_ []PendingWithdrawal, err error) {
	if a == nil {
		return nil, ErrClientNotAvailable
	}

	start := time.Now()
	defer func() {
		a.metrics.observePendingWithdrawals(ctx, time.Since(start), err)
	}()

	log.Infow("querying pending withdrawals", "signer", signer)
	withdrawals, err := a.client.PendingWithdrawals(ctx, signer)
	if err != nil {
		log.Errorw("querying pending withdrawals", "err", err, "signer", signer)
		return nil, err
	}

	log.Debugw("pending withdrawals found", "signer", signer, "count", len(withdrawals))
	result := make([]PendingWithdrawal, len(withdrawals))
	for i, w := range withdrawals {
		result[i] = PendingWithdrawal{
			Signer:             w.Signer,
			Amount:             w.Amount,
			RequestedTimestamp: w.RequestedTimestamp,
			AvailableTimestamp: w.AvailableTimestamp,
		}
	}
	return result, nil
}
