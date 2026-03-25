package fibre

import (
	"context"

	sdktypes "github.com/cosmos/cosmos-sdk/types"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/fibre"
	"github.com/celestiaorg/celestia-node/state/txclient"
)

// Service wraps the fibre Client and AccountClient to implement Module.
type Service struct {
	client  *fibre.Client
	account *fibre.AccountClient
}

func NewService(client *fibre.Client) *Service {
	return &Service{
		client:  client,
		account: client.Account(),
	}
}

func (s *Service) Upload(ctx context.Context, ns libshare.Namespace, data []byte) (*fibre.UploadResult, error) {
	return s.client.Upload(ctx, ns, data)
}

func (s *Service) Get(ctx context.Context, ns libshare.Namespace, commitment []byte) (*fibre.GetBlobResponse, error) {
	return s.client.Get(ctx, ns, commitment)
}

func (s *Service) QueryEscrowAccount(ctx context.Context, signer string) (*fibre.EscrowAccount, error) {
	return s.account.QueryEscrowAccount(ctx, signer)
}

func (s *Service) Deposit(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error {
	return s.account.Deposit(ctx, amount, cfg)
}

func (s *Service) Withdraw(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error {
	return s.account.Withdraw(ctx, amount, cfg)
}

func (s *Service) PendingWithdrawals(ctx context.Context, signer string) ([]fibre.PendingWithdrawal, error) {
	return s.account.PendingWithdrawals(ctx, signer)
}
