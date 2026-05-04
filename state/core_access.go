package state

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/feegrant"
	"github.com/cometbft/cometbft/crypto/merkle"
	"github.com/cometbft/cometbft/proto/tendermint/crypto"
	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	v2bank "github.com/cosmos/cosmos-sdk/x/bank/migrations/v2"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-app/v9/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v9/pkg/user"
	libhead "github.com/celestiaorg/go-header"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/state/txclient"
)

var tracer = otel.Tracer("state/service")

var (
	ErrInvalidAmount = errors.New("state: amount must be greater than zero")
	ErrInvalidHeight = errors.New("state: height must be positive and fit in int64")
	log              = logging.Logger("state")
)

type TxConfig = txclient.TxConfig

type TxClient interface {
	SubmitMessage(context.Context, sdktypes.Msg, *txclient.TxConfig) (*user.TxResponse, error)
	SubmitPayForBlob(context.Context, []*libshare.Blob, sdktypes.AccAddress, *txclient.TxConfig) (*user.TxResponse, error)
}

// CoreAccessor implements service over a gRPC connection
// with a celestia-core node.
type CoreAccessor struct {
	ctx    context.Context
	cancel context.CancelFunc

	txClient TxClient

	keyring keyring.Keyring

	// TODO @renaynay: clean this up -- only one!!!!
	defaultSignerAccount string
	defaultSignerAddress AccAddress

	getter libhead.Head[*header.ExtendedHeader]

	stakingCli      stakingtypes.QueryClient
	distributionCli distributiontypes.QueryClient
	feeGrantCli     feegrant.QueryClient
	abciQueryCli    tmservice.ServiceClient

	prt *merkle.ProofRuntime

	coreConn *grpc.ClientConn
	network  string

	// these fields are mutatable and thus need to be protected by a mutex
	lock            sync.Mutex
	lastPayForBlob  int64
	payForBlobCount int64
}

// NewCoreAccessor dials the given celestia-core endpoint and
// constructs and returns a new CoreAccessor (state service) with the active
// connection.
func NewCoreAccessor(
	client TxClient,
	keyring keyring.Keyring,
	keyname string,
	getter libhead.Head[*header.ExtendedHeader],
	conn *grpc.ClientConn,
	network string,
) (*CoreAccessor, error) {
	// create verifier
	prt := merkle.DefaultProofRuntime()
	prt.RegisterOpDecoder(storetypes.ProofOpIAVLCommitment, storetypes.CommitmentOpDecoder)
	prt.RegisterOpDecoder(storetypes.ProofOpSimpleMerkleCommitment, storetypes.CommitmentOpDecoder)

	rec, err := keyring.Key(keyname)
	if err != nil {
		return nil, err
	}
	addr, err := rec.GetAddress()
	if err != nil {
		return nil, err
	}

	ca := &CoreAccessor{
		txClient:             client,
		keyring:              keyring,
		defaultSignerAccount: keyname,
		defaultSignerAddress: addr,
		getter:               getter,
		prt:                  prt,
		coreConn:             conn,
		network:              network,
	}
	return ca, nil
}

func (ca *CoreAccessor) Start(ctx context.Context) error {
	ca.ctx, ca.cancel = context.WithCancel(context.Background())
	// create the staking query client
	ca.stakingCli = stakingtypes.NewQueryClient(ca.coreConn)
	ca.distributionCli = distributiontypes.NewQueryClient(ca.coreConn)
	ca.feeGrantCli = feegrant.NewQueryClient(ca.coreConn)
	// create ABCI query client
	ca.abciQueryCli = tmservice.NewServiceClient(ca.coreConn)
	resp, err := ca.abciQueryCli.GetNodeInfo(ctx, &tmservice.GetNodeInfoRequest{})
	if err != nil {
		return fmt.Errorf("failed to get node info: %w", err)
	}

	defaultNetwork := resp.GetDefaultNodeInfo().GetNetwork()
	if defaultNetwork != ca.network {
		return fmt.Errorf("wrong network in core.ip endpoint, expected %s, got %s", ca.network, defaultNetwork)
	}
	return nil
}

func (ca *CoreAccessor) Stop(_ context.Context) error {
	ca.cancel()
	return nil
}

// SubmitPayForBlob builds, signs, and synchronously submits a MsgPayForBlob with additional
// options defined in `TxConfig`. It blocks until the transaction is committed and returns the
// TxResponse. The user can specify additional options that can bee applied to the Tx.
func (ca *CoreAccessor) SubmitPayForBlob(
	ctx context.Context,
	libBlobs []*libshare.Blob,
	cfg *TxConfig,
) (_ *TxResponse, err error) {
	span := trace.SpanFromContext(ctx)
	// span will be set to noopSpan if SubmitPayForBlob is called directly
	if !span.SpanContext().IsValid() {
		ctx, span = tracer.Start(ctx, "state/submit")
		defer func() {
			utils.SetStatusAndEnd(span, err)
		}()
	}

	if len(libBlobs) == 0 {
		err = errors.New("state: no blobs provided")
		return nil, err
	}

	span.SetAttributes(
		attribute.Int("amount", len(libBlobs)),
		attribute.Float64("gas-price", cfg.GasPrice()),
		attribute.Int64("gas-limit", int64(cfg.GasLimit())),
		attribute.String("signer", cfg.SignerAddress()),
		attribute.String("fee-granter", cfg.FeeGranterAddress()),
		attribute.String("key-name", cfg.KeyName()),
		attribute.Int("tx-priority", int(cfg.TxPriority())),
	)

	ids := make([]string, len(libBlobs))
	dataLengths := make([]int, len(libBlobs))
	for i := range libBlobs {
		if err := libBlobs[i].Namespace().ValidateForBlob(); err != nil {
			return nil, fmt.Errorf("not allowed namespace %s were used to build the blob", libBlobs[i].Namespace().ID())
		}

		ids[i] = libBlobs[i].Namespace().String()
		dataLengths[i] = libBlobs[i].DataLen()
	}

	span.SetAttributes(attribute.StringSlice("namespaces", ids))
	span.SetAttributes(attribute.IntSlice("blob-data-lengths", dataLengths))

	// get signer address
	author, err := ca.getTxAuthorAccAddress(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get tx author address: %w", err)
	}

	response, err := ca.txClient.SubmitPayForBlob(ctx, libBlobs, author, cfg)
	if err != nil {
		return nil, err
	}

	// metrics should only be counted on a successful PFB tx
	if response.Code == 0 {
		ca.markSuccessfulPFB()
	}

	resp := convertToSdkTxResponse(response)
	span.SetAttributes(
		attribute.Int64("height", resp.Height),
		attribute.String("hash", resp.TxHash),
		attribute.Int("code", int(resp.Code)),
		attribute.Int64("gas-wanted", resp.GasWanted),
		attribute.Int64("gas-used", resp.GasUsed),
	)
	return resp, nil
}

func (ca *CoreAccessor) AccountAddress(context.Context) (Address, error) {
	return Address{ca.defaultSignerAddress}, nil
}

func (ca *CoreAccessor) Balance(ctx context.Context) (*Balance, error) {
	return ca.BalanceForAddress(ctx, Address{ca.defaultSignerAddress})
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
	prefixedAccountKey := append(v2bank.CreateAccountBalancesPrefix(addr.Bytes()), []byte(appconsts.BondDenom)...)
	req := &tmservice.ABCIQueryRequest{
		Data: prefixedAccountKey,
		// TODO @renayay: once https://github.com/cosmos/cosmos-sdk/pull/12674 is merged, use const instead
		Path:   fmt.Sprintf("store/%s/key", banktypes.StoreKey),
		Height: int64(head.Height() - 1),
		Prove:  true,
	}

	result, err := ca.abciQueryCli.ABCIQuery(ctx, req)
	if err != nil || result.GetCode() != 0 {
		err = fmt.Errorf(
			"failed to query for balance: %w; result log: %s; result code: %d",
			err, result.GetLog(), result.GetCode(),
		)
		return nil, err
	}

	// unmarshal balance information
	value := result.GetValue()
	// if the value returned is empty, the account balance does not yet exist
	if len(value) == 0 {
		log.Errorf("balance for account %s does not exist at block height %d", addr.String(), head.Height()-1)
		return &Balance{
			Denom:  appconsts.BondDenom,
			Amount: sdkmath.NewInt(0),
		}, nil
	}
	coin, ok := sdkmath.NewIntFromString(string(value))
	if !ok {
		return nil, fmt.Errorf("cannot convert %s into sdktypes.Int", string(value))
	}

	if result.GetProofOps() == nil {
		return nil, fmt.Errorf("failed to get proofs for balance of address %s", addr.String())
	}

	// verify balance
	proofOps := &crypto.ProofOps{
		Ops: make([]crypto.ProofOp, len(result.ProofOps.Ops)),
	}
	for i, proofOp := range result.ProofOps.Ops {
		proofOps.Ops[i] = crypto.ProofOp{
			Type: proofOp.Type,
			Key:  proofOp.Key,
			Data: proofOp.Data,
		}
	}

	err = ca.prt.VerifyValueFromKeys(
		proofOps,
		head.AppHash,
		[][]byte{
			[]byte(banktypes.StoreKey),
			prefixedAccountKey,
		}, value)
	if err != nil {
		return nil, err
	}

	return &Balance{
		Denom:  appconsts.BondDenom,
		Amount: coin,
	}, nil
}

func (ca *CoreAccessor) Transfer(
	ctx context.Context,
	addr AccAddress,
	amount Int,
	cfg *TxConfig,
) (*TxResponse, error) {
	if amount.IsNil() || !amount.IsPositive() {
		return nil, ErrInvalidAmount
	}

	signer, err := ca.getTxAuthorAccAddress(cfg)
	if err != nil {
		return nil, err
	}

	coins := sdktypes.NewCoins(sdktypes.NewCoin(appconsts.BondDenom, amount))
	msg := banktypes.NewMsgSend(signer, addr, coins)
	response, err := ca.txClient.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return nil, err
	}
	return convertToSdkTxResponse(response), nil
}

func (ca *CoreAccessor) CancelUnbondingDelegation(
	ctx context.Context,
	valAddr ValAddress,
	amount,
	height Int,
	cfg *TxConfig,
) (*TxResponse, error) {
	if amount.IsNil() || !amount.IsPositive() {
		return nil, ErrInvalidAmount
	}
	if height.IsNil() || !height.IsPositive() || !height.IsInt64() {
		return nil, ErrInvalidHeight
	}

	signer, err := ca.getTxAuthorAccAddress(cfg)
	if err != nil {
		return nil, err
	}

	coins := sdktypes.NewCoin(appconsts.BondDenom, amount)
	msg := stakingtypes.NewMsgCancelUnbondingDelegation(signer.String(), valAddr.String(), height.Int64(), coins)
	response, err := ca.txClient.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return nil, err
	}
	return convertToSdkTxResponse(response), nil
}

func (ca *CoreAccessor) BeginRedelegate(
	ctx context.Context,
	srcValAddr,
	dstValAddr ValAddress,
	amount Int,
	cfg *TxConfig,
) (*TxResponse, error) {
	if amount.IsNil() || !amount.IsPositive() {
		return nil, ErrInvalidAmount
	}

	signer, err := ca.getTxAuthorAccAddress(cfg)
	if err != nil {
		return nil, err
	}

	coins := sdktypes.NewCoin(appconsts.BondDenom, amount)
	msg := stakingtypes.NewMsgBeginRedelegate(signer.String(), srcValAddr.String(), dstValAddr.String(), coins)

	response, err := ca.txClient.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return nil, err
	}
	return convertToSdkTxResponse(response), nil
}

func (ca *CoreAccessor) Undelegate(
	ctx context.Context,
	delAddr ValAddress,
	amount Int,
	cfg *TxConfig,
) (*TxResponse, error) {
	if amount.IsNil() || !amount.IsPositive() {
		return nil, ErrInvalidAmount
	}

	signer, err := ca.getTxAuthorAccAddress(cfg)
	if err != nil {
		return nil, err
	}

	coins := sdktypes.NewCoin(appconsts.BondDenom, amount)
	msg := stakingtypes.NewMsgUndelegate(signer.String(), delAddr.String(), coins)

	response, err := ca.txClient.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return nil, err
	}
	return convertToSdkTxResponse(response), nil
}

func (ca *CoreAccessor) Delegate(
	ctx context.Context,
	delAddr ValAddress,
	amount Int,
	cfg *TxConfig,
) (*TxResponse, error) {
	if amount.IsNil() || !amount.IsPositive() {
		return nil, ErrInvalidAmount
	}

	signer, err := ca.getTxAuthorAccAddress(cfg)
	if err != nil {
		return nil, err
	}

	coins := sdktypes.NewCoin(appconsts.BondDenom, amount)
	msg := stakingtypes.NewMsgDelegate(signer.String(), delAddr.String(), coins)

	response, err := ca.txClient.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return nil, err
	}
	return convertToSdkTxResponse(response), nil
}

func (ca *CoreAccessor) WithdrawDelegatorReward(
	ctx context.Context,
	valAddr ValAddress,
	cfg *TxConfig,
) (*TxResponse, error) {
	signer, err := ca.getTxAuthorAccAddress(cfg)
	if err != nil {
		return nil, err
	}

	msg := distributiontypes.NewMsgWithdrawDelegatorReward(signer.String(), valAddr.String())
	response, err := ca.txClient.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return nil, err
	}
	return convertToSdkTxResponse(response), nil
}

func (ca *CoreAccessor) QueryDelegationRewards(
	ctx context.Context,
	valAddr ValAddress,
) (*distributiontypes.QueryDelegationRewardsResponse, error) {
	delAddr := ca.defaultSignerAddress
	return ca.distributionCli.DelegationRewards(ctx, &distributiontypes.QueryDelegationRewardsRequest{
		DelegatorAddress: delAddr.String(),
		ValidatorAddress: valAddr.String(),
	})
}

func (ca *CoreAccessor) QueryDelegation(
	ctx context.Context,
	valAddr ValAddress,
) (*stakingtypes.QueryDelegationResponse, error) {
	delAddr := ca.defaultSignerAddress
	return ca.stakingCli.Delegation(ctx, &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: delAddr.String(),
		ValidatorAddr: valAddr.String(),
	})
}

func (ca *CoreAccessor) QueryUnbonding(
	ctx context.Context,
	valAddr ValAddress,
) (*stakingtypes.QueryUnbondingDelegationResponse, error) {
	delAddr := ca.defaultSignerAddress
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
	delAddr := ca.defaultSignerAddress
	return ca.stakingCli.Redelegations(ctx, &stakingtypes.QueryRedelegationsRequest{
		DelegatorAddr:    delAddr.String(),
		SrcValidatorAddr: srcValAddr.String(),
		DstValidatorAddr: dstValAddr.String(),
	})
}

func (ca *CoreAccessor) GrantFee(
	ctx context.Context,
	grantee AccAddress,
	amount Int,
	cfg *TxConfig,
) (*TxResponse, error) {
	granter, err := ca.getTxAuthorAccAddress(cfg)
	if err != nil {
		return nil, err
	}

	allowance := &feegrant.BasicAllowance{}
	if !amount.IsZero() {
		// set spend limit
		allowance.SpendLimit = sdktypes.NewCoins(sdktypes.NewCoin(appconsts.BondDenom, amount))
	}

	msg, err := feegrant.NewMsgGrantAllowance(allowance, granter, grantee)
	if err != nil {
		return nil, err
	}

	response, err := ca.txClient.SubmitMessage(ctx, msg, cfg)
	if err != nil {
		return nil, err
	}
	return convertToSdkTxResponse(response), nil
}

func (ca *CoreAccessor) RevokeGrantFee(
	ctx context.Context,
	grantee AccAddress,
	cfg *TxConfig,
) (*TxResponse, error) {
	granter, err := ca.getTxAuthorAccAddress(cfg)
	if err != nil {
		return nil, err
	}

	msg := feegrant.NewMsgRevokeAllowance(granter, grantee)

	response, err := ca.txClient.SubmitMessage(ctx, &msg, cfg)
	if err != nil {
		return nil, err
	}
	return convertToSdkTxResponse(response), nil
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

// getTxAuthorAccAddress attempts to return the account address of the intended
// author of the transaction.
// TODO @renaynay:
//
//	tx client requires an `Account Name` which is basically keyname in the keyring but when building a sdk msg,
//	we need an sdk.AccAddress. We should consolidate the type TxClient needs so we don't have random account
//	determination logic everywhere that tries to parse back and forth between
//	key names / account address / account name etc.
func (ca *CoreAccessor) getTxAuthorAccAddress(cfg *TxConfig) (AccAddress, error) {
	switch {
	case cfg.SignerAddress() != "":
		return txclient.ParseAccAddressFromString(cfg.SignerAddress())
	case cfg.KeyName() != "" && cfg.KeyName() != ca.defaultSignerAccount:
		return txclient.ParseAccountKey(ca.keyring, cfg.KeyName())
	default:
		return ca.defaultSignerAddress, nil
	}
}

// convertToTxResponse converts the user.TxResponse to sdk.TxResponse.
// This is a temporary workaround in order to avoid breaking the api.
func convertToSdkTxResponse(resp *user.TxResponse) *TxResponse {
	return &TxResponse{
		Code:   resp.Code,
		TxHash: resp.TxHash,
		Height: resp.Height,
	}
}
