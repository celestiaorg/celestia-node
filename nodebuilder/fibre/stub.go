package fibre

import (
	"context"
	"errors"

	sdktypes "github.com/cosmos/cosmos-sdk/types"

	appfibre "github.com/celestiaorg/celestia-app/v8/fibre"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/fibre"
	"github.com/celestiaorg/celestia-node/state/txclient"
)

var errFibreNotAvailable = errors.New("fibre module is not available: node is not connected to a core endpoint")

type stubbedFibreModule struct{}

func (s *stubbedFibreModule) Submit(
	context.Context,
	libshare.Namespace,
	[]byte,
	*txclient.TxConfig,
) (*SubmitResult, error) {
	return nil, errFibreNotAvailable
}

func (s *stubbedFibreModule) Upload(
	context.Context,
	libshare.Namespace,
	[]byte,
	*txclient.TxConfig,
) (*UploadResult, error) {
	return nil, errFibreNotAvailable
}

func (s *stubbedFibreModule) Download(context.Context, appfibre.BlobID) (*GetBlobResult, error) {
	return nil, errFibreNotAvailable
}

func (s *stubbedFibreModule) QueryEscrowAccount(context.Context, string) (*fibre.EscrowAccount, error) {
	return nil, errFibreNotAvailable
}

func (s *stubbedFibreModule) Deposit(context.Context, sdktypes.Coin, *txclient.TxConfig) error {
	return errFibreNotAvailable
}

func (s *stubbedFibreModule) Withdraw(context.Context, sdktypes.Coin, *txclient.TxConfig) error {
	return errFibreNotAvailable
}

func (s *stubbedFibreModule) PendingWithdrawals(context.Context, string) ([]fibre.PendingWithdrawal, error) {
	return nil, errFibreNotAvailable
}
