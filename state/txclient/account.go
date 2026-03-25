package txclient

import (
	"context"
	"fmt"

	sdktypes "github.com/cosmos/cosmos-sdk/types"

	fibretypes "github.com/celestiaorg/celestia-app/v8/x/fibre/types"
)

func (c *TxClient) QueryEscrowAccount(ctx context.Context, signer string) (*fibretypes.EscrowAccount, error) {
	queryClient := fibretypes.NewQueryClient(c.coreConns[0])
	resp, err := queryClient.EscrowAccount(ctx, &fibretypes.QueryEscrowAccountRequest{
		Signer: signer,
	})
	if err != nil {
		return nil, fmt.Errorf("querying escrow account: %w", err)
	}
	if !resp.Found {
		return nil, fmt.Errorf("escrow account not found for signer %s", signer)
	}
	return resp.EscrowAccount, nil
}

func (c *TxClient) Deposit(ctx context.Context, amount sdktypes.Coin, cfg *TxConfig) error {
	if err := c.setupClients(); err != nil {
		return err
	}

	signer, err := c.getTxAuthorAccAddress(cfg)
	if err != nil {
		return fmt.Errorf("getting signer address: %w", err)
	}

	msg := &fibretypes.MsgDepositToEscrow{
		Signer: signer.String(),
		Amount: amount,
	}
	_, err = c.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return fmt.Errorf("depositing to escrow: %w", err)
	}
	return nil
}

func (c *TxClient) Withdraw(ctx context.Context, amount sdktypes.Coin, cfg *TxConfig) error {
	if err := c.setupClients(); err != nil {
		return err
	}

	signer, err := c.getTxAuthorAccAddress(cfg)
	if err != nil {
		return fmt.Errorf("getting signer address: %w", err)
	}

	msg := &fibretypes.MsgRequestWithdrawal{
		Signer: signer.String(),
		Amount: amount,
	}
	_, err = c.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return fmt.Errorf("requesting withdrawal: %w", err)
	}
	return nil
}

// getTxAuthorAccAddress resolves the signer address from cfg, falling back
// to the default signer.
func (c *TxClient) getTxAuthorAccAddress(cfg *TxConfig) (sdktypes.AccAddress, error) {
	switch {
	case cfg != nil && cfg.SignerAddress() != "":
		return ParseAccAddressFromString(cfg.SignerAddress())
	case cfg != nil && cfg.KeyName() != "" && cfg.KeyName() != c.defaultSignerAccount:
		return ParseAccountKey(c.keyring, cfg.KeyName())
	default:
		return c.defaultSignerAddress, nil
	}
}

func (c *TxClient) PendingWithdrawals(ctx context.Context, signer string) ([]fibretypes.Withdrawal, error) {
	queryClient := fibretypes.NewQueryClient(c.coreConns[0])
	resp, err := queryClient.Withdrawals(ctx, &fibretypes.QueryWithdrawalsRequest{
		Signer: signer,
	})
	if err != nil {
		return nil, fmt.Errorf("querying withdrawals: %w", err)
	}
	return resp.Withdrawals, nil
}
