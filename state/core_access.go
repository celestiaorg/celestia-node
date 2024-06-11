package state

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	sdkErrors "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/api/tendermint/abci"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	auth "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	logging "github.com/ipfs/go-log/v2"
	"github.com/tendermint/tendermint/crypto/merkle"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	apperrors "github.com/celestiaorg/celestia-app/app/errors"
	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/user"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
)

const (
	// gasMultiplier is used to increase gas limit in case if tx has additional options.
	gasMultiplier = 1.1
	maxRetries    = 5
)

var (
	ErrInvalidAmount = errors.New("state: amount must be greater than zero")

	log = logging.Logger("state")
)

// CoreAccessor implements service over a gRPC connection
// with a celestia-core node.
type CoreAccessor struct {
	ctx    context.Context
	cancel context.CancelFunc

	keyring keyring.Keyring
	addr    sdktypes.AccAddress
	// TODO: (@cmwaters) - once multiple keys within a signer is supported,
	// this will no longer be necessary.
	// ref: https://github.com/celestiaorg/celestia-app/issues/3259
	signerMu sync.Mutex
	signer   *user.Signer

	getter libhead.Head[*header.ExtendedHeader]

	stakingCli stakingtypes.QueryClient
	rpcCli     rpcclient.ABCIClient

	prt *merkle.ProofRuntime

	coreConn *grpc.ClientConn
	coreIP   string
	rpcPort  string
	grpcPort string

	// these fields are mutatable and thus need to be protected by a mutex
	lock            sync.Mutex
	lastPayForBlob  int64
	payForBlobCount int64
	// minGasPrice is the minimum gas price that the node will accept.
	// NOTE: just because the first node accepts the transaction, does not mean it
	// will find a proposer that does accept the transaction. Better would be
	// to set a global min gas price that correct processes conform to.
	minGasPrice float64
}

// NewCoreAccessor dials the given celestia-core endpoint and
// constructs and returns a new CoreAccessor (state service) with the active
// connection.
func NewCoreAccessor(
	keyring keyring.Keyring,
	keyname string,
	getter libhead.Head[*header.ExtendedHeader],
	coreIP,
	rpcPort string,
	grpcPort string,
) (*CoreAccessor, error) {
	// create verifier
	prt := merkle.DefaultProofRuntime()
	prt.RegisterOpDecoder(storetypes.ProofOpIAVLCommitment, storetypes.CommitmentOpDecoder)
	prt.RegisterOpDecoder(storetypes.ProofOpSimpleMerkleCommitment, storetypes.CommitmentOpDecoder)

	record, err := keyring.Key(keyname)
	if err != nil {
		return nil, fmt.Errorf("getting key %s: %w", keyname, err)
	}
	addr, err := record.GetAddress()
	if err != nil {
		return nil, fmt.Errorf("getting address for key %s: %w", keyname, err)
	}

	return &CoreAccessor{
		keyring:  keyring,
		addr:     addr,
		getter:   getter,
		coreIP:   coreIP,
		rpcPort:  rpcPort,
		grpcPort: grpcPort,
		prt:      prt,
	}, nil
}

func (ca *CoreAccessor) Start(ctx context.Context) error {
	if ca.coreConn != nil {
		return fmt.Errorf("core-access: already connected to core endpoint")
	}
	ca.ctx, ca.cancel = context.WithCancel(context.Background())

	// dial given celestia-core endpoint
	endpoint := fmt.Sprintf("%s:%s", ca.coreIP, ca.grpcPort)
	client, err := grpc.DialContext(
		ctx,
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	ca.coreConn = client

	// create the staking query client
	stakingCli := stakingtypes.NewQueryClient(ca.coreConn)
	ca.stakingCli = stakingCli
	// create ABCI query client
	cli, err := http.New(fmt.Sprintf("http://%s:%s", ca.coreIP, ca.rpcPort), "/websocket")
	if err != nil {
		return err
	}
	ca.rpcCli = cli

	ca.signer, err = ca.setupSigner(ctx)
	if err != nil {
		log.Warnw("failed to set up signer, check if node's account is funded", "err", err)
	}

	ca.minGasPrice, err = ca.queryMinimumGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("querying minimum gas price: %w", err)
	}

	return nil
}

func (ca *CoreAccessor) Stop(context.Context) error {
	if ca.cancel == nil {
		log.Warn("core accessor already stopped")
		return nil
	}
	if ca.coreConn == nil {
		log.Warn("no connection found to close")
		return nil
	}
	defer ca.cancelCtx()

	// close out core connection
	err := ca.coreConn.Close()
	if err != nil {
		return err
	}

	ca.coreConn = nil
	return nil
}

func (ca *CoreAccessor) cancelCtx() {
	ca.cancel()
	ca.cancel = nil
}

// SubmitPayForBlob builds, signs, and synchronously submits a MsgPayForBlob. It blocks until the
// transaction is committed and returns the TxResponse. If gasLim is set to 0, the method will
// automatically estimate the gas limit. If the fee is negative, the method will use the nodes min
// gas price multiplied by the gas limit.
func (ca *CoreAccessor) SubmitPayForBlob(
	ctx context.Context,
	fee Int,
	gasLim uint64,
	blobs []*blob.Blob,
) (*TxResponse, error) {
	signer, err := ca.getSigner(ctx)
	if err != nil {
		return nil, err
	}

	if len(blobs) == 0 {
		return nil, errors.New("state: no blobs provided")
	}

	appblobs := make([]*apptypes.Blob, len(blobs))
	for i := range blobs {
		if err := blobs[i].Namespace().ValidateForBlob(); err != nil {
			return nil, err
		}
		appblobs[i] = &blobs[i].Blob
	}

	// we only estimate gas if the user wants us to (by setting the gasLim to 0). In the future we may
	// want to make these arguments optional.
	if gasLim == 0 {
		blobSizes := make([]uint32, len(blobs))
		for i, blob := range blobs {
			blobSizes[i] = uint32(len(blob.Data))
		}

		// TODO (@cmwaters): the default gas per byte and the default tx size cost per byte could be changed
		// through governance. This section could be more robust by tracking these values and adjusting the
		// gas limit accordingly (as is done for the gas price)
		gasLim = apptypes.EstimateGas(blobSizes, appconsts.DefaultGasPerBlobByte, auth.DefaultTxSizeCostPerByte)
	}

	minGasPrice := ca.getMinGasPrice()

	// set the fee for the user as the minimum gas price multiplied by the gas limit
	estimatedFee := false
	if fee.IsNegative() {
		estimatedFee = true
		fee = sdktypes.NewInt(int64(math.Ceil(minGasPrice * float64(gasLim))))
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		response, err := signer.SubmitPayForBlob(
			ctx,
			appblobs,
			user.SetGasLimit(gasLim),
			withFee(fee),
		)

		// the node is capable of changing the min gas price at any time so we must be able to detect it and
		// update our version accordingly
		if apperrors.IsInsufficientMinGasPrice(err) && estimatedFee {
			// The error message contains enough information to parse the new min gas price
			minGasPrice, err = apperrors.ParseInsufficientMinGasPrice(err, minGasPrice, gasLim)
			if err != nil {
				return nil, fmt.Errorf("parsing insufficient min gas price error: %w", err)
			}
			ca.setMinGasPrice(minGasPrice)
			lastErr = err
			// update the fee to retry again
			fee = sdktypes.NewInt(int64(math.Ceil(minGasPrice * float64(gasLim))))
			continue
		}

		// metrics should only be counted on a successful PFD tx
		if err == nil && response.Code == 0 {
			ca.markSuccessfulPFB()
		}

		if response != nil && response.Code != 0 {
			err = errors.Join(err, sdkErrors.ABCIError(response.Codespace, response.Code, response.Logs.String()))
		}
		return response, err
	}
	return nil, fmt.Errorf("failed to submit blobs after %d attempts: %w", maxRetries, lastErr)
}

func (ca *CoreAccessor) AccountAddress(context.Context) (Address, error) {
	return Address{ca.addr}, nil
}

func (ca *CoreAccessor) Balance(ctx context.Context) (*Balance, error) {
	return ca.BalanceForAddress(ctx, Address{ca.addr})
}

func (ca *CoreAccessor) BalanceForAddress(ctx context.Context, addr Address) (*Balance, error) {
	head, err := ca.getter.Head(ctx)
	if err != nil {
		return nil, err
	}
	// construct an ABCI query for the height at head-1 because
	// the AppHash contained in the head is actually the state root
	// after applying the transactions contained in the previous block.
	// TODO @renaynay: once https://github.com/cosmos/cosmos-sdk/pull/12674 is merged, use this method
	// instead
	prefixedAccountKey := append(banktypes.CreateAccountBalancesPrefix(addr.Bytes()), []byte(app.BondDenom)...)
	abciReq := abci.RequestQuery{
		// TODO @renayay: once https://github.com/cosmos/cosmos-sdk/pull/12674 is merged, use const instead
		Path:   fmt.Sprintf("store/%s/key", banktypes.StoreKey),
		Height: int64(head.Height() - 1),
		Data:   prefixedAccountKey,
		Prove:  true,
	}
	opts := rpcclient.ABCIQueryOptions{
		Height: abciReq.GetHeight(),
		Prove:  abciReq.GetProve(),
	}
	result, err := ca.rpcCli.ABCIQueryWithOptions(ctx, abciReq.GetPath(), abciReq.GetData(), opts)
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
		log.Errorf("balance for account %s does not exist at block height %d", addr.String(), head.Height()-1)
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
	err = ca.prt.VerifyValueFromKeys(
		result.Response.GetProofOps(),
		head.AppHash,
		[][]byte{
			[]byte(banktypes.StoreKey),
			prefixedAccountKey,
		}, value)
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
	addr AccAddress,
	amount,
	fee Int,
	gasLim uint64,
) (*TxResponse, error) {
	signer, err := ca.getSigner(ctx)
	if err != nil {
		return nil, err
	}

	if amount.IsNil() || amount.Int64() <= 0 {
		return nil, ErrInvalidAmount
	}

	coins := sdktypes.NewCoins(sdktypes.NewCoin(app.BondDenom, amount))
	msg := banktypes.NewMsgSend(signer.Address(), addr, coins)
	if gasLim == 0 {
		var err error
		gasLim, err = signer.EstimateGas(ctx, []sdktypes.Msg{msg})
		if err != nil {
			return nil, fmt.Errorf("estimating gas: %w", err)
		}
	}

	return signer.SubmitTx(ctx, []sdktypes.Msg{msg}, user.SetGasLimit(gasLim), user.SetFee(fee.Uint64()))
}

func (ca *CoreAccessor) CancelUnbondingDelegation(
	ctx context.Context,
	valAddr ValAddress,
	amount,
	height,
	fee Int,
	gasLim uint64,
) (*TxResponse, error) {
	signer, err := ca.getSigner(ctx)
	if err != nil {
		return nil, err
	}

	if amount.IsNil() || amount.Int64() <= 0 {
		return nil, ErrInvalidAmount
	}

	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgCancelUnbondingDelegation(signer.Address(), valAddr, height.Int64(), coins)
	if gasLim == 0 {
		var err error
		gasLim, err = signer.EstimateGas(ctx, []sdktypes.Msg{msg})
		if err != nil {
			return nil, fmt.Errorf("estimating gas: %w", err)
		}
	}

	return signer.SubmitTx(ctx, []sdktypes.Msg{msg}, user.SetGasLimit(gasLim), user.SetFee(fee.Uint64()))
}

func (ca *CoreAccessor) BeginRedelegate(
	ctx context.Context,
	srcValAddr,
	dstValAddr ValAddress,
	amount,
	fee Int,
	gasLim uint64,
) (*TxResponse, error) {
	signer, err := ca.getSigner(ctx)
	if err != nil {
		return nil, err
	}

	if amount.IsNil() || amount.Int64() <= 0 {
		return nil, ErrInvalidAmount
	}

	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgBeginRedelegate(signer.Address(), srcValAddr, dstValAddr, coins)
	if gasLim == 0 {
		var err error
		gasLim, err = signer.EstimateGas(ctx, []sdktypes.Msg{msg})
		if err != nil {
			return nil, fmt.Errorf("estimating gas: %w", err)
		}
	}

	return signer.SubmitTx(ctx, []sdktypes.Msg{msg}, user.SetGasLimit(gasLim), user.SetFee(fee.Uint64()))
}

func (ca *CoreAccessor) Undelegate(
	ctx context.Context,
	delAddr ValAddress,
	amount,
	fee Int,
	gasLim uint64,
) (*TxResponse, error) {
	signer, err := ca.getSigner(ctx)
	if err != nil {
		return nil, err
	}

	if amount.IsNil() || amount.Int64() <= 0 {
		return nil, ErrInvalidAmount
	}

	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgUndelegate(signer.Address(), delAddr, coins)
	if gasLim == 0 {
		var err error
		gasLim, err = signer.EstimateGas(ctx, []sdktypes.Msg{msg})
		if err != nil {
			return nil, fmt.Errorf("estimating gas: %w", err)
		}
	}
	return signer.SubmitTx(ctx, []sdktypes.Msg{msg}, user.SetGasLimit(gasLim), user.SetFee(fee.Uint64()))
}

func (ca *CoreAccessor) Delegate(
	ctx context.Context,
	delAddr ValAddress,
	amount Int,
	fee Int,
	gasLim uint64,
) (*TxResponse, error) {
	signer, err := ca.getSigner(ctx)
	if err != nil {
		return nil, err
	}

	if amount.IsNil() || amount.Int64() <= 0 {
		return nil, ErrInvalidAmount
	}

	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgDelegate(signer.Address(), delAddr, coins)
	if gasLim == 0 {
		var err error
		gasLim, err = signer.EstimateGas(ctx, []sdktypes.Msg{msg})
		if err != nil {
			return nil, fmt.Errorf("estimating gas: %w", err)
		}
	}
	return signer.SubmitTx(ctx, []sdktypes.Msg{msg}, user.SetGasLimit(gasLim), user.SetFee(fee.Uint64()))
}

func (ca *CoreAccessor) QueryDelegation(
	ctx context.Context,
	valAddr ValAddress,
) (*stakingtypes.QueryDelegationResponse, error) {
	delAddr := ca.addr
	return ca.stakingCli.Delegation(ctx, &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: delAddr.String(),
		ValidatorAddr: valAddr.String(),
	})
}

func (ca *CoreAccessor) QueryUnbonding(
	ctx context.Context,
	valAddr ValAddress,
) (*stakingtypes.QueryUnbondingDelegationResponse, error) {
	delAddr := ca.addr
	return ca.stakingCli.UnbondingDelegation(ctx, &stakingtypes.QueryUnbondingDelegationRequest{
		DelegatorAddr: delAddr.String(),
		ValidatorAddr: valAddr.String(),
	})
}

func (ca *CoreAccessor) QueryRedelegations(
	ctx context.Context,
	srcValAddr,
	dstValAddr ValAddress,
) (*stakingtypes.QueryRedelegationsResponse, error) {
	delAddr := ca.addr
	return ca.stakingCli.Redelegations(ctx, &stakingtypes.QueryRedelegationsRequest{
		DelegatorAddr:    delAddr.String(),
		SrcValidatorAddr: srcValAddr.String(),
		DstValidatorAddr: dstValAddr.String(),
	})
}

func (ca *CoreAccessor) LastPayForBlob() int64 {
	ca.lock.Lock()
	defer ca.lock.Unlock()
	return ca.lastPayForBlob
}

func (ca *CoreAccessor) PayForBlobCount() int64 {
	ca.lock.Lock()
	defer ca.lock.Unlock()
	return ca.payForBlobCount
}

func (ca *CoreAccessor) markSuccessfulPFB() {
	ca.lock.Lock()
	defer ca.lock.Unlock()
	ca.lastPayForBlob = time.Now().UnixMilli()
	ca.payForBlobCount++
}

func (ca *CoreAccessor) setMinGasPrice(minGasPrice float64) {
	ca.lock.Lock()
	defer ca.lock.Unlock()
	ca.minGasPrice = minGasPrice
}

func (ca *CoreAccessor) getMinGasPrice() float64 {
	ca.lock.Lock()
	defer ca.lock.Unlock()
	return ca.minGasPrice
}

// QueryMinimumGasPrice returns the minimum gas price required by the node.
func (ca *CoreAccessor) queryMinimumGasPrice(
	ctx context.Context,
) (float64, error) {
	rsp, err := nodeservice.NewServiceClient(ca.coreConn).Config(ctx, &nodeservice.ConfigRequest{})
	if err != nil {
		return 0, err
	}

	coins, err := sdktypes.ParseDecCoins(rsp.MinimumGasPrice)
	if err != nil {
		return 0, err
	}
	return coins.AmountOf(app.BondDenom).MustFloat64(), nil
}

// getSigner returns the signer if it has already been constructed, otherwise
// it will attempt to set it up. The signer can only be constructed if the account
// exists / is funded.
func (ca *CoreAccessor) getSigner(ctx context.Context) (*user.Signer, error) {
	ca.signerMu.Lock()
	defer ca.signerMu.Unlock()

	if ca.signer != nil {
		return ca.signer, nil
	}

	var err error
	ca.signer, err = ca.setupSigner(ctx)
	return ca.signer, err
}

func (ca *CoreAccessor) setupSigner(ctx context.Context) (*user.Signer, error) {
	encCfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	return user.SetupSigner(ctx, ca.keyring, ca.coreConn, ca.addr, encCfg)
}

func withFee(fee Int) user.TxOption {
	gasFee := sdktypes.NewCoins(sdktypes.NewCoin(app.BondDenom, fee))
	return user.SetFeeAmount(gasFee)
}
