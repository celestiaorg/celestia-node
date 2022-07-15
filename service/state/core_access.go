package state

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/api/tendermint/abci"
	"github.com/cosmos/cosmos-sdk/types"
	sdk_types "github.com/cosmos/cosmos-sdk/types"
	sdk_errors "github.com/cosmos/cosmos-sdk/types/errors"
	sdk_tx "github.com/cosmos/cosmos-sdk/types/tx"
	bank_types "github.com/cosmos/cosmos-sdk/x/bank/types"
	proof_utils "github.com/cosmos/ibc-go/v4/modules/core/23-commitment/types"
	sdk_abci "github.com/tendermint/tendermint/abci/types"
	rpc_client "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/x/payment"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/nmt/namespace"
)

// CoreAccessor implements Accessor over a gRPC connection
// with a celestia-core node.
type CoreAccessor struct {
	signer *apptypes.KeyringSigner
	getter header.Getter

	queryCli bank_types.QueryClient
	rpcCli   rpc_client.ABCIClient

	coreConn *grpc.ClientConn
	coreIP   string
	rpcPort  string
	grpcPort string
}

// NewCoreAccessor dials the given celestia-core endpoint and
// constructs and returns a new CoreAccessor with the active
// connection.
func NewCoreAccessor(
	signer *apptypes.KeyringSigner,
	getter header.Getter,
	coreIP,
	rpcPort string,
	grpcPort string,
) *CoreAccessor {
	return &CoreAccessor{
		signer:   signer,
		getter:   getter,
		coreIP:   coreIP,
		rpcPort:  rpcPort,
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
	queryCli := bank_types.NewQueryClient(ca.coreConn)
	ca.queryCli = queryCli
	// create ABCI query client
	cli, err := http.New(fmt.Sprintf("http://%s:%s", ca.coreIP, ca.rpcPort))
	if err != nil {
		return err
	}
	ca.rpcCli = cli
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
	req := &bank_types.QueryBalanceRequest{
		Address: addr.String(),
		Denom:   app.BondDenom,
	}

	resp, err := ca.queryCli.Balance(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("querying client for balance: %s", err.Error())
	}

	return resp.Balance, nil
}

func (ca *CoreAccessor) VerifiedBalance(ctx context.Context) (*Balance, error) {
	addr, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	return ca.VerifiedBalanceForAddress(ctx, addr)
}

func (ca *CoreAccessor) VerifiedBalanceForAddress(ctx context.Context, addr Address) (*Balance, error) {
	head, err := ca.getter.Head(ctx)
	if err != nil {
		return nil, err
	}
	// construct an ABCI query for the height at head-1 because
	// the AppHash contained in the head is actually the hash of
	// the transactions contained in the previous blocks.
	// TODO @renaynay: make PR on app to create convenience method for constructing this key
	prefixedAccountKey := append(bank_types.CreateAccountBalancesPrefix(addr.Bytes()), []byte(app.BondDenom)...)
	abciReq := abci.RequestQuery{
		// TODO @renayay: make PR on app to extract this into const
		Path:   fmt.Sprintf("store/%s/key", bank_types.StoreKey),
		Height: head.Height - 1,
		Data:   prefixedAccountKey,
		Prove:  true,
	}
	opts := rpc_client.ABCIQueryOptions{
		Height: abciReq.Height,
		Prove:  abciReq.Prove,
	}
	result, err := ca.rpcCli.ABCIQueryWithOptions(ctx, abciReq.Path, abciReq.Data, opts)
	if err != nil {
		return nil, err
	}
	if !result.Response.IsOK() {
		return nil, sdkErrorToGRPCError(result.Response)
	}
	// unmarshal balance information
	value := result.Response.Value
	coin, ok := sdk_types.NewIntFromString(string(value))
	if !ok {
		return nil, fmt.Errorf("cannot convert %s into sdk_types.Int", string(value))
	}
	// convert proofs into a more digestible format
	merkleproof, err := proof_utils.ConvertProofs(result.Response.GetProofOps())
	if err != nil {
		return nil, err
	}
	root := proof_utils.NewMerkleRoot(head.AppHash)
	// VerifyMembership expects the path as:
	// []string{<store key of module>, <actual key corresponding to requested value>}
	path := proof_utils.NewMerklePath(bank_types.StoreKey, string(prefixedAccountKey))
	err = merkleproof.VerifyMembership(proof_utils.GetSDKSpecs(), root, path, value)
	if err != nil {
		return nil, err
	}
	return &Balance{
		Denom:  app.BondDenom,
		Amount: coin,
	}, nil
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
	msg := bank_types.NewMsgSend(from, to, coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}

func sdkErrorToGRPCError(resp sdk_abci.ResponseQuery) error {
	switch resp.Code {
	case sdk_errors.ErrInvalidRequest.ABCICode():
		return status.Error(codes.InvalidArgument, resp.Log)
	case sdk_errors.ErrUnauthorized.ABCICode():
		return status.Error(codes.Unauthenticated, resp.Log)
	case sdk_errors.ErrKeyNotFound.ABCICode():
		return status.Error(codes.NotFound, resp.Log)
	default:
		return status.Error(codes.Unknown, resp.Log)
	}
}
