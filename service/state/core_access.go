package state

import (
	"context"
	"fmt"

	sdk_tx "github.com/cosmos/cosmos-sdk/types/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/tendermint/spm/cosmoscmd"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-app/app"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
)

// CoreAccessor implements Accessor over an RPC connection
// with a celestia-core node.
type CoreAccessor struct {
	signer *apptypes.KeyringSigner
	encCfg cosmoscmd.EncodingConfig

	coreEndpoint string
	coreConn     *grpc.ClientConn
}

// NewCoreAccessor dials the given celestia-core endpoint and
// constructs and returns a new CoreAccessor with the active
// connection.
func NewCoreAccessor(
	signer *apptypes.KeyringSigner,
	encCfg cosmoscmd.EncodingConfig,
	endpoint string,
) (*CoreAccessor, error) {
	return &CoreAccessor{
		signer:       signer,
		encCfg:       encCfg,
		coreEndpoint: endpoint,
	}, nil
}

func (ca *CoreAccessor) Start(ctx context.Context) error {
	if ca.coreConn != nil {
		return fmt.Errorf("core-access: already connected to core endpoint")
	}
	// dial given celestia-core endpoint
	client, err := grpc.DialContext(ctx, ca.coreEndpoint, grpc.WithBlock())
	if err != nil {
		return err
	}
	ca.coreConn = client
	return nil
}

func (ca *CoreAccessor) Stop(_ context.Context) error {
	if ca.coreConn == nil {
		return fmt.Errorf("core-access: no connection found to close")
	}
	err := ca.coreConn.Close()
	if err != nil {
		return err
	}
	ca.coreConn = nil
	return nil
}

func (ca *CoreAccessor) Balance(ctx context.Context) (*Balance, error) {
	return ca.BalanceForAddress(ctx, ca.signer.GetSignerInfo().GetAddress().String())
}

func (ca *CoreAccessor) BalanceForAddress(ctx context.Context, addr string) (*Balance, error) {
	queryCli := banktypes.NewQueryClient(ca.coreConn)

	balReq := &banktypes.QueryBalanceRequest{
		Address: addr,
		Denom:   app.DisplayDenom,
	}

	balResp, err := queryCli.Balance(ctx, balReq)
	if err != nil {
		return nil, err
	}

	return balResp.Balance, nil
}

func (ca *CoreAccessor) SubmitTx(ctx context.Context, tx Tx) (*TxResponse, error) {
	txResp, err := apptypes.BroadcastTx(ctx, ca.coreConn, sdk_tx.BroadcastMode_BROADCAST_MODE_SYNC, tx)
	if err != nil {
		return nil, err
	}
	return txResp.TxResponse, nil
}
