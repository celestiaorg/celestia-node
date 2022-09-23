package state

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/api/tendermint/abci"
	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	logging "github.com/ipfs/go-log/v2"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/x/payment"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/nmt/namespace"

	"github.com/celestiaorg/celestia-node/header"
)

var log = logging.Logger("state")

// coreAccessor implements service over a gRPC connection
// with a celestia-core node.
type coreAccessor struct {
	ctx    context.Context
	cancel context.CancelFunc

	signer *apptypes.KeyringSigner
	getter header.Head

	queryCli   banktypes.QueryClient
	stakingCli stakingtypes.QueryClient
	rpcCli     rpcclient.ABCIClient

	coreConn *grpc.ClientConn
	coreIP   string
	rpcPort  string
	grpcPort string
}

// NewCoreAccessor dials the given celestia-core endpoint and
// constructs and returns a new coreAccessor (state service) with the active
// connection.
func NewCoreAccessor(
	signer *apptypes.KeyringSigner,
	getter header.Head,
	coreIP,
	rpcPort string,
	grpcPort string,
) *coreAccessor { //nolint:revive
	return &coreAccessor{
		signer:   signer,
		getter:   getter,
		coreIP:   coreIP,
		rpcPort:  rpcPort,
		grpcPort: grpcPort,
	}
}

func (ca *coreAccessor) Start(ctx context.Context) error {
	ca.ctx, ca.cancel = context.WithCancel(ctx)
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
	// create the staking query client
	stakingCli := stakingtypes.NewQueryClient(ca.coreConn)
	ca.stakingCli = stakingCli
	// create ABCI query client
	cli, err := http.New(fmt.Sprintf("http://%s:%s", ca.coreIP, ca.rpcPort), "/websocket")
	if err != nil {
		return err
	}
	ca.rpcCli = cli
	return nil
}

func (ca *coreAccessor) Stop(context.Context) error {
	defer ca.cancel()
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

func (ca *coreAccessor) constructSignedTx(
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

func (ca *coreAccessor) SubmitPayForData(
	ctx context.Context,
	nID namespace.ID,
	data []byte,
	gasLim uint64,
) (*TxResponse, error) {
	return payment.SubmitPayForData(ctx, ca.signer, ca.coreConn, nID, data, gasLim)
}

func (ca *coreAccessor) Balance(ctx context.Context) (*Balance, error) {
	addr, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	return ca.BalanceForAddress(ctx, addr)
}

func (ca *coreAccessor) BalanceForAddress(ctx context.Context, addr Address) (*Balance, error) {
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
		log.Errorf("balance for account %s does not exist at block height %d", addr.String(), head.Height-1)
		return &Balance{
			Denom:  app.BondDenom,
			Amount: sdktypes.NewInt(0),
		}, nil
	}
	coin, ok := sdktypes.NewIntFromString(string(value))
	if !ok {
		return nil, fmt.Errorf("cannot convert %s into sdktypes.Int", string(value))
	}
	// verify balance
	path := fmt.Sprintf("/%s/%s", banktypes.StoreKey, string(prefixedAccountKey))
	prt := rootmulti.DefaultProofRuntime()
	err = prt.VerifyValue(result.Response.GetProofOps(), head.AppHash, path, value)
	if err != nil {
		return nil, err
	}

	return &Balance{
		Denom:  app.BondDenom,
		Amount: coin,
	}, nil
}

func (ca *coreAccessor) SubmitTx(ctx context.Context, tx Tx) (*TxResponse, error) {
	txResp, err := apptypes.BroadcastTx(ctx, ca.coreConn, sdktx.BroadcastMode_BROADCAST_MODE_BLOCK, tx)
	if err != nil {
		return nil, err
	}
	return txResp.TxResponse, nil
}

func (ca *coreAccessor) SubmitTxWithBroadcastMode(
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

func (ca *coreAccessor) Transfer(
	ctx context.Context,
	addr AccAddress,
	amount Int,
	gasLim uint64,
) (*TxResponse, error) {
	from, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	coins := sdktypes.NewCoins(sdktypes.NewCoin(app.BondDenom, amount))
	msg := banktypes.NewMsgSend(from, addr, coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}

func (ca *coreAccessor) CancelUnbondingDelegation(
	ctx context.Context,
	valAddr ValAddress,
	amount,
	height Int,
	gasLim uint64,
) (*TxResponse, error) {
	from, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgCancelUnbondingDelegation(from, valAddr, height.Int64(), coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}

func (ca *coreAccessor) BeginRedelegate(
	ctx context.Context,
	srcValAddr,
	dstValAddr ValAddress,
	amount Int,
	gasLim uint64,
) (*TxResponse, error) {
	from, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgBeginRedelegate(from, srcValAddr, dstValAddr, coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}

func (ca *coreAccessor) Undelegate(
	ctx context.Context,
	delAddr ValAddress,
	amount Int,
	gasLim uint64,
) (*TxResponse, error) {
	from, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgUndelegate(from, delAddr, coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}

func (ca *coreAccessor) Delegate(
	ctx context.Context,
	delAddr ValAddress,
	amount Int,
	gasLim uint64,
) (*TxResponse, error) {
	from, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgDelegate(from, delAddr, coins)
	signedTx, err := ca.constructSignedTx(ctx, msg, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}
	return ca.SubmitTx(ctx, signedTx)
}

func (ca *coreAccessor) QueryDelegation(
	ctx context.Context,
	valAddr ValAddress,
) (*stakingtypes.QueryDelegationResponse, error) {
	delAddr, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	return ca.stakingCli.Delegation(ctx, &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: delAddr.String(),
		ValidatorAddr: valAddr.String(),
	})
}

func (ca *coreAccessor) QueryUnbonding(
	ctx context.Context,
	valAddr ValAddress,
) (*stakingtypes.QueryUnbondingDelegationResponse, error) {
	delAddr, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	return ca.stakingCli.UnbondingDelegation(ctx, &stakingtypes.QueryUnbondingDelegationRequest{
		DelegatorAddr: delAddr.String(),
		ValidatorAddr: valAddr.String(),
	})
}
func (ca *coreAccessor) QueryRedelegations(
	ctx context.Context,
	srcValAddr,
	dstValAddr ValAddress,
) (*stakingtypes.QueryRedelegationsResponse, error) {
	delAddr, err := ca.signer.GetSignerInfo().GetAddress()
	if err != nil {
		return nil, err
	}
	return ca.stakingCli.Redelegations(ctx, &stakingtypes.QueryRedelegationsRequest{
		DelegatorAddr:    delAddr.String(),
		SrcValidatorAddr: srcValAddr.String(),
		DstValidatorAddr: dstValAddr.String(),
	})
}

func (ca *coreAccessor) IsStopped() bool {
	return ca.ctx.Err() != nil
}
