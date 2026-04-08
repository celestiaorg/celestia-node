package fibre

import (
	"context"
	"fmt"
	"time"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc"

	fibretypes "github.com/celestiaorg/celestia-app/v8/x/fibre/types"

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
	client *txclient.TxClient

	conns   []*grpc.ClientConn
	metrics *accountMetrics
}

type Option func(*AccountClient)

func WithAdditionalConnections(conns ...*grpc.ClientConn) Option {
	return func(ac *AccountClient) {
		ac.conns = append(ac.conns, conns...)
	}
}

func NewAccountClient(client *txclient.TxClient, conn *grpc.ClientConn, opts ...Option) *AccountClient {
	acc := &AccountClient{client: client}
	acc.conns = append(acc.conns, conn)
	for _, opt := range opts {
		opt(acc)
	}
	return acc
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

	var resp *fibretypes.QueryEscrowAccountResponse
	for _, conn := range a.conns {
		queryClient := fibretypes.NewQueryClient(conn)
		resp, err = queryClient.EscrowAccount(ctx, &fibretypes.QueryEscrowAccountRequest{
			Signer: signer,
		})
		if err != nil {
			log.Warnw("querying escrow account", "error", err)
			continue
		}

		if resp.Found {
			break
		}
		log.Warnw("escrow account not found for signer", "signer", signer)
	}

	if resp == nil {
		return nil, fmt.Errorf("querying escrow account: %w", err)
	}
	if !resp.Found {
		return nil, fmt.Errorf("escrow account not found for signer:%s", signer)
	}

	log.Debugw("escrow account found",
		"signer", resp.EscrowAccount.Signer,
		"balance", resp.EscrowAccount.Balance,
		"available-balance", resp.EscrowAccount.AvailableBalance,
	)
	return &EscrowAccount{
		Signer:           resp.EscrowAccount.Signer,
		Balance:          resp.EscrowAccount.Balance,
		AvailableBalance: resp.EscrowAccount.AvailableBalance,
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

	signer, err := a.client.GetTxAuthorAccAddress(cfg)
	if err != nil {
		return fmt.Errorf("getting signer address: %w", err)
	}

	msg := &fibretypes.MsgDepositToEscrow{
		Signer: signer.String(),
		Amount: amount,
	}

	_, err = a.client.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return fmt.Errorf("depositing to escrow: %w", err)
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

	signer, err := a.client.GetTxAuthorAccAddress(cfg)
	if err != nil {
		return fmt.Errorf("getting signer address: %w", err)
	}

	msg := &fibretypes.MsgRequestWithdrawal{
		Signer: signer.String(),
		Amount: amount,
	}

	_, err = a.client.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return fmt.Errorf("requesting withdrawal: %w", err)
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

	var resp *fibretypes.QueryWithdrawalsResponse

	for _, conn := range a.conns {
		queryClient := fibretypes.NewQueryClient(conn)
		resp, err = queryClient.Withdrawals(ctx, &fibretypes.QueryWithdrawalsRequest{
			Signer: signer,
		})
		if err == nil {
			break
		}

		if err != nil {
			log.Warnw("querying pending withdrawals", "err", err, "signer", signer)
		}
	}

	if resp == nil {
		log.Errorw("querying pending withdrawals", "err", err, "signer", signer)
		return nil, err
	}

	log.Debugw("pending withdrawals found", "signer", signer, "count", len(resp.Withdrawals))
	result := make([]PendingWithdrawal, len(resp.Withdrawals))
	for i, w := range resp.Withdrawals {
		result[i] = PendingWithdrawal{
			Signer:             w.Signer,
			Amount:             w.Amount,
			RequestedTimestamp: w.RequestedTimestamp,
			AvailableTimestamp: w.AvailableTimestamp,
		}
	}
	return result, nil
}
