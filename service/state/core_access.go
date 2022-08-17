package state

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/api/tendermint/abci"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	proofutils "github.com/cosmos/ibc-go/v4/modules/core/23-commitment/types"
	logging "github.com/ipfs/go-log/v2"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/x/payment"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/nmt/namespace"
)

var log = logging.Logger("state")

// CoreAccessor implements Accessor over a gRPC connection
// with a celestia-core node.
type CoreAccessor struct {
	signer *apptypes.KeyringSigner
	getter header.Getter

	queryCli banktypes.QueryClient
	rpcCli   rpcclient.ABCIClient

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
	queryCli := banktypes.NewQueryClient(ca.coreConn)
	ca.queryCli = queryCli
	// create ABCI query client
	cli, err := http.New(fmt.Sprintf("http://%s:%s", ca.coreIP, ca.rpcPort), "/websocket")
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
	msg sdktypes.Msg,
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
	head, err := ca.getter.Head(ctx)
	if err != nil {
		return nil, err
	}
	// construct an ABCI query for the height at head-1 because
	// the AppHash contained in the head is actually the state root
	// after applying the transactions contained in the previous block.
	// TODO @renaynay: once https://github.com/cosmos/cosmos-sdk/pull/12674 is merged, use this method instead
	prefixedAccountKey := append(banktypes.CreateAccountBalancesPrefix(addr.Bytes()), []byte(app.BondDenom)...)
	abciReq := abci.RequestQuery{
		// TODO @renayay: once https://github.com/cosmos/cosmos-sdk/pull/12674 is merged, use const instead
		Path:   fmt.Sprintf("store/%s/key", banktypes.StoreKey),
		Height: head.Height - 1,
		Data:   prefixedAccountKey,
		Prove:  true,
	}
	opts := rpcclient.ABCIQueryOptions{
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
	// if the value returned is empty, the account balance does not yet exist
	if len(value) == 0 {
		log.Errorf("balance for account %s does not exist at block height %d", addr.String(), head.Height)
		return &Balance{
			Denom:  app.BondDenom,
			Amount: sdktypes.NewInt(0),
		}, nil
	}
	coin, ok := sdktypes.NewIntFromString(string(value))
	if !ok {
		return nil, fmt.Errorf("cannot convert %s into sdktypes.Int", string(value))
	}
	// convert proofs into a more digestible format
	merkleproof, err := proofutils.ConvertProofs(result.Response.GetProofOps())
	if err != nil {
		return nil, err
	}
	root := proofutils.NewMerkleRoot(head.AppHash)
	// VerifyMembership expects the path as:
	// []string{<store key of module>, <actual key corresponding to requested value>}
	path := proofutils.NewMerklePath(banktypes.StoreKey, string(prefixedAccountKey))
	err = merkleproof.VerifyMembership(proofutils.GetSDKSpecs(), root, path, value)
	if err != nil {
		return nil, err
	}
	return &Balance{
		Denom:  app.BondDenom,
		Amount: coin,
	}, nil
}

func (ca *CoreAccessor) SubmitTx(ctx context.Context, tx Tx) (*TxResponse, error) {
	txResp, err := apptypes.BroadcastTx(ctx, ca.coreConn, sdktx.BroadcastMode_BROADCAST_MODE_BLOCK, tx)
	if err != nil {
		return nil, err
	}
	return txResp.TxResponse, nil
}

func (ca *CoreAccessor) SubmitTxWithBroadcastMode(
	ctx context.Context,
	tx Tx,
	mode sdktx.BroadcastMode,
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
	to, ok := addr.(sdktypes.AccAddress)
	if !ok {
		return nil, fmt.Errorf("state: unsupported address type")
	}
	from, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	coins := sdktypes.NewCoins(sdktypes.NewCoin(app.BondDenom, amount))
	msg := banktypes.NewMsgSend(from, to, coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}

func (ca *CoreAccessor) CancelUnbondingDelegation(
	ctx context.Context,
	valAddr Address,
	amount,
	height Int,
	gasLim uint64,
) (*TxResponse, error) {
	validator, ok := valAddr.(sdktypes.ValAddress)
	if !ok {
		return nil, fmt.Errorf("state: unsupported address type")
	}
	from, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgCancelUnbondingDelegation(from, validator, height.Int64(), coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}

func (ca *CoreAccessor) BeginRedelegate(
	ctx context.Context,
	srcValAddr,
	dstValAddr Address,
	amount Int,
	gasLim uint64,
) (*TxResponse, error) {
	srcValidator, ok := srcValAddr.(sdktypes.ValAddress)
	if !ok {
		return nil, fmt.Errorf("state: unsupported address type")
	}
	dstValidator, ok := dstValAddr.(sdktypes.ValAddress)
	if !ok {
		return nil, fmt.Errorf("state: unsupported address type")
	}
	from, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgBeginRedelegate(from, srcValidator, dstValidator, coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}

func (ca *CoreAccessor) Undelegate(
	ctx context.Context,
	delAddr Address,
	amount Int,
	gasLim uint64,
) (*TxResponse, error) {
	delegate, ok := delAddr.(sdktypes.ValAddress)
	if !ok {
		return nil, fmt.Errorf("state: unsupported address type")
	}
	from, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgUndelegate(from, delegate, coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}

func (ca *CoreAccessor) Delegate(
	ctx context.Context,
	delAddr Address,
	amount Int,
	gasLim uint64,
) (*TxResponse, error) {
	delegate, ok := delAddr.(sdktypes.ValAddress)
	if !ok {
		return nil, fmt.Errorf("state: unsupported address type")
	}
	from, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgDelegate(from, delegate, coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}
