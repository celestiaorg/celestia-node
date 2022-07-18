package state

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/types"
	sdk_tx "github.com/cosmos/cosmos-sdk/types/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/x/payment"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/nmt/namespace"
)

// CoreAccessor implements Accessor over a gRPC connection
// with a celestia-core node.
type CoreAccessor struct {
	signer *apptypes.KeyringSigner

	coreIP   string
	grpcPort string
	coreConn *grpc.ClientConn
	queryCli banktypes.QueryClient
}

// NewCoreAccessor dials the given celestia-core endpoint and
// constructs and returns a new CoreAccessor with the active
// connection.
func NewCoreAccessor(
	signer *apptypes.KeyringSigner,
	coreIP,
	grpcPort string,
) *CoreAccessor {
	return &CoreAccessor{
		signer:   signer,
		coreIP:   coreIP,
		grpcPort: grpcPort,
	}
}

func (ca *CoreAccessor) Start(ctx context.Context) error {
	if ca.coreConn != nil {
		return fmt.Errorf("core-access: already connected to core endpoint")
	}
	// dial given celestia-core endpoint
	endpoint := fmt.Sprintf("%s:%s", ca.coreIP, ca.grpcPort)
	client, err := grpc.DialContext(ctx, endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	ca.coreConn = client
	// create the query client
	queryCli := banktypes.NewQueryClient(ca.coreConn)
	ca.queryCli = queryCli
	return nil
}

func (ca *CoreAccessor) Stop(context.Context) error {
	if ca.coreConn == nil {
		return fmt.Errorf("core-access: no connection found to close")
	}
	// close out core connection
	err := ca.coreConn.Close()
	if err != nil {
		return err
	}
	ca.coreConn = nil
	ca.queryCli = nil
	return nil
}

func (ca *CoreAccessor) constructSignedTx(
	ctx context.Context,
	msg types.Msg,
	opts ...apptypes.TxBuilderOption,
) ([]byte, error) {
	// should be called first in order to make a valid tx
	err := ca.signer.QueryAccountNumber(ctx, ca.coreConn)
	if err != nil {
		return nil, err
	}

	tx, err := ca.signer.BuildSignedTx(ca.signer.NewTxBuilder(opts...), msg)
	if err != nil {
		return nil, err
	}
	return ca.signer.EncodeTx(tx)
}

func (ca *CoreAccessor) SubmitPayForData(
	ctx context.Context,
	nID namespace.ID,
	data []byte,
	gasLim uint64,
) (*TxResponse, error) {
	return payment.SubmitPayForData(ctx, ca.signer, ca.coreConn, nID, data, gasLim)
}

func (ca *CoreAccessor) Balance(ctx context.Context) (*Balance, error) {
	addr, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	return ca.BalanceForAddress(ctx, addr)
}

func (ca *CoreAccessor) BalanceForAddress(ctx context.Context, addr Address) (*Balance, error) {
	req := &banktypes.QueryBalanceRequest{
		Address: addr.String(),
		Denom:   app.BondDenom,
	}

	resp, err := ca.queryCli.Balance(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("querying client for balance: %s", err.Error())
	}

	return resp.Balance, nil
}

func (ca *CoreAccessor) SubmitTx(ctx context.Context, tx Tx) (*TxResponse, error) {
	txResp, err := apptypes.BroadcastTx(ctx, ca.coreConn, sdk_tx.BroadcastMode_BROADCAST_MODE_BLOCK, tx)
	if err != nil {
		return nil, err
	}
	return txResp.TxResponse, nil
}

func (ca *CoreAccessor) SubmitTxWithBroadcastMode(
	ctx context.Context,
	tx Tx,
	mode sdk_tx.BroadcastMode,
) (*TxResponse, error) {
	txResp, err := apptypes.BroadcastTx(ctx, ca.coreConn, mode, tx)
	if err != nil {
		return nil, err
	}
	return txResp.TxResponse, nil
}

func (ca *CoreAccessor) Transfer(
	ctx context.Context,
	addr Address,
	amount Int,
	gasLim uint64,
) (*TxResponse, error) {
	to, ok := addr.(types.AccAddress)
	if !ok {
		return nil, fmt.Errorf("state: unsupported address type")
	}
	from, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	coins := types.NewCoins(types.NewCoin(app.BondDenom, amount))
	msg := banktypes.NewMsgSend(from, to, coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}
