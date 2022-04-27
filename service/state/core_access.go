package state

import (
	"context"
	"fmt"

	sdk_tx "github.com/cosmos/cosmos-sdk/types/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/x/payment"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/nmt/namespace"
)

// CoreAccessor implements Accessor over an RPC connection
// with a celestia-core node.
type CoreAccessor struct {
	signer *apptypes.KeyringSigner

	coreEndpoint string
	coreConn     *grpc.ClientConn
	queryCli     banktypes.QueryClient
}

// NewCoreAccessor dials the given celestia-core endpoint and
// constructs and returns a new CoreAccessor with the active
// connection.
func NewCoreAccessor(
	signer *apptypes.KeyringSigner,
	endpoint string,
) *CoreAccessor {
	return &CoreAccessor{
		signer:       signer,
		coreEndpoint: endpoint,
	}
}

func (ca *CoreAccessor) Start(ctx context.Context) error {
	if ca.coreConn != nil {
		return fmt.Errorf("core-access: already connected to core endpoint")
	}
	// dial given celestia-core endpoint
	client, err := grpc.DialContext(ctx, ca.coreEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

func (ca *CoreAccessor) SubmitPayForData(
	ctx context.Context,
	nID namespace.ID,
	data []byte,
	gasLim uint64,
) (*TxResponse, error) {
	return payment.SubmitPayForData(ctx, ca.signer, ca.coreConn, nID, data, gasLim)
}

func (ca *CoreAccessor) Balance(ctx context.Context) (*Balance, error) {
	return ca.BalanceForAddress(ctx, ca.signer.GetSignerInfo().GetAddress())
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
