package state

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	sdkErrors "cosmossdk.io/errors"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	logging "github.com/ipfs/go-log/v2"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v2/app"
	"github.com/celestiaorg/celestia-app/v2/app/encoding"
	apperrors "github.com/celestiaorg/celestia-app/v2/app/errors"
	"github.com/celestiaorg/celestia-app/v2/pkg/user"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
)

const (
	maxRetries = 5
)

var (
	ErrInvalidAmount = errors.New("state: amount must be greater than zero")

	log = logging.Logger("state")
)

// Option is the functional option that is applied to the coreAccessor instance
// to configure parameters.
type Option func(ca *CoreAccessor)

// CoreAccessor implements service over a gRPC connection
// with a celestia-core node.
type CoreAccessor struct {
	ctx    context.Context
	cancel context.CancelFunc

	keyring keyring.Keyring
	client  *user.TxClient

	// TODO: remove in scope of https://github.com/celestiaorg/celestia-node/issues/3515
	defaultSignerAccount string
	defaultSignerAddress AccAddress

	getter libhead.Head[*header.ExtendedHeader]

	stakingCli   stakingtypes.QueryClient
	feeGrantCli  feegrant.QueryClient
	abciQueryCli tmservice.ServiceClient

	prt *merkle.ProofRuntime

	coreConn *grpc.ClientConn
	coreIP   string
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
	grpcPort string,
	options ...Option,
) (*CoreAccessor, error) {
	// create verifier
	prt := merkle.DefaultProofRuntime()
	prt.RegisterOpDecoder(storetypes.ProofOpIAVLCommitment, storetypes.CommitmentOpDecoder)
	prt.RegisterOpDecoder(storetypes.ProofOpSimpleMerkleCommitment, storetypes.CommitmentOpDecoder)

	ca := &CoreAccessor{
		keyring:              keyring,
		defaultSignerAccount: keyname,
		getter:               getter,
		coreIP:               coreIP,
		grpcPort:             grpcPort,
		prt:                  prt,
	}

	for _, opt := range options {
		opt(ca)
	}
	return ca, nil
}

func (ca *CoreAccessor) Start(ctx context.Context) error {
	if ca.coreConn != nil {
		return fmt.Errorf("core-access: already connected to core endpoint")
	}
	ca.ctx, ca.cancel = context.WithCancel(context.Background())

	// dial given celestia-core endpoint
	endpoint := fmt.Sprintf("%s:%s", ca.coreIP, ca.grpcPort)
	client, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	// this ensures we can't start the node without core connection
	client.Connect()
	if !client.WaitForStateChange(ctx, connectivity.Ready) {
		// hits the case when context is canceled
		return fmt.Errorf("couldn't connect to core endpoint(%s): %w", endpoint, ctx.Err())
	}

	ca.coreConn = client

	// create the staking query client
	ca.stakingCli = stakingtypes.NewQueryClient(ca.coreConn)
	ca.feeGrantCli = feegrant.NewQueryClient(ca.coreConn)

	// create ABCI query client
	ca.abciQueryCli = tmservice.NewServiceClient(ca.coreConn)

	// set up signer to handle tx submission
	ca.client, err = ca.setupTxClient(ctx, ca.defaultSignerAccount)
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

// SubmitPayForBlob builds, signs, and synchronously submits a MsgPayForBlob with additional
// options defined in `TxConfig`. It blocks until the transaction is committed and returns the
// TxResponse. The user can specify additional options that can bee applied to the Tx.
func (ca *CoreAccessor) SubmitPayForBlob(
	ctx context.Context,
	appblobs []*Blob,
	cfg *TxConfig,
) (*TxResponse, error) {
	if len(appblobs) == 0 {
		return nil, errors.New("state: no blobs provided")
	}

	var feeGrant user.TxOption
	if cfg.FeeGranterAddress() != "" {
		granter, err := parseAccAddressFromString(cfg.FeeGranterAddress())
		if err != nil {
			return nil, err
		}
		feeGrant = user.SetFeeGranter(granter)
	}

	gas := cfg.GasLimit()
	if gas == 0 {
		blobSizes := make([]uint32, len(appblobs))
		for i, blob := range appblobs {
			blobSizes[i] = uint32(len(blob.GetData()))
		}
		gas = estimateGasForBlobs(blobSizes)
	}

	gasPrice := cfg.GasPrice()
	if cfg.GasPrice() == DefaultGasPrice {
		gasPrice = ca.getMinGasPrice()
	}

	signer, err := ca.getSigner(cfg)
	if err != nil {
		return nil, err
	}

	accName := ca.defaultSignerAccount
	if !signer.Equals(ca.defaultSignerAddress) {
		account := ca.client.AccountByAddress(signer)
		if account == nil {
			return nil, fmt.Errorf("account for signer %s not found", signer)
		}
		accName = account.Name()
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		opts := []user.TxOption{user.SetGasLimitAndGasPrice(gas, gasPrice)}
		if feeGrant != nil {
			opts = append(opts, feeGrant)
		}
		response, err := ca.client.BroadcastPayForBlobWithAccount(
			ctx,
			accName,
			appblobs,
			opts...,
		)
		if err != nil {
			return nil, err
		}

		// TODO @vgonkivs: remove me to achieve async blob submission
		response, err = ca.client.ConfirmTx(ctx, response.TxHash)
		// the node is capable of changing the min gas price at any time so we must be able to detect it and
		// update our version accordingly
		if apperrors.IsInsufficientMinGasPrice(err) {
			// The error message contains enough information to parse the new min gas price
			gasPrice, err = apperrors.ParseInsufficientMinGasPrice(err, gasPrice, gas)
			if err != nil {
				return nil, fmt.Errorf("parsing insufficient min gas price error: %w", err)
			}

			ca.setMinGasPrice(gasPrice)
			lastErr = err
			continue
		}

		// metrics should only be counted on a successful PFD tx
		if err == nil && response.Code == 0 {
			ca.markSuccessfulPFB()
		}

		if response != nil && response.Code != 0 {
			err = errors.Join(err, sdkErrors.ABCIError(response.Codespace, response.Code, response.Logs.String()))
		}
		return unsetTx(response), err
	}
	return nil, fmt.Errorf("failed to submit blobs after %d attempts: %w", maxRetries, lastErr)
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
	prefixedAccountKey := append(banktypes.CreateAccountBalancesPrefix(addr.Bytes()), []byte(app.BondDenom)...)
	req := &tmservice.ABCIQueryRequest{
		Data: prefixedAccountKey,
		// TODO @renayay: once https://github.com/cosmos/cosmos-sdk/pull/12674 is merged, use const instead
		Path:   fmt.Sprintf("store/%s/key", banktypes.StoreKey),
		Height: int64(head.Height() - 1),
		Prove:  true,
	}

	result, err := ca.abciQueryCli.ABCIQuery(ctx, req)
	if err != nil || result.GetCode() != 0 {
		err = fmt.Errorf("failed to query for balance: %w; result log: %s", err, result.GetLog())
		return nil, err
	}

	// unmarshal balance information
	value := result.GetValue()
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
		Denom:  app.BondDenom,
		Amount: coin,
	}, nil
}

func (ca *CoreAccessor) Transfer(
	ctx context.Context,
	addr AccAddress,
	amount Int,
	cfg *TxConfig,
) (*TxResponse, error) {
	if amount.IsNil() || amount.Int64() <= 0 {
		return nil, ErrInvalidAmount
	}

	signer, err := ca.getSigner(cfg)
	if err != nil {
		return nil, err
	}

	coins := sdktypes.NewCoins(sdktypes.NewCoin(app.BondDenom, amount))
	msg := banktypes.NewMsgSend(signer, addr, coins)
	return ca.submitMsg(ctx, msg, cfg)
}

func (ca *CoreAccessor) CancelUnbondingDelegation(
	ctx context.Context,
	valAddr ValAddress,
	amount,
	height Int,
	cfg *TxConfig,
) (*TxResponse, error) {
	if amount.IsNil() || amount.Int64() <= 0 {
		return nil, ErrInvalidAmount
	}

	signer, err := ca.getSigner(cfg)
	if err != nil {
		return nil, err
	}

	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgCancelUnbondingDelegation(signer, valAddr, height.Int64(), coins)
	return ca.submitMsg(ctx, msg, cfg)
}

func (ca *CoreAccessor) BeginRedelegate(
	ctx context.Context,
	srcValAddr,
	dstValAddr ValAddress,
	amount Int,
	cfg *TxConfig,
) (*TxResponse, error) {
	if amount.IsNil() || amount.Int64() <= 0 {
		return nil, ErrInvalidAmount
	}

	signer, err := ca.getSigner(cfg)
	if err != nil {
		return nil, err
	}

	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgBeginRedelegate(signer, srcValAddr, dstValAddr, coins)
	return ca.submitMsg(ctx, msg, cfg)
}

func (ca *CoreAccessor) Undelegate(
	ctx context.Context,
	delAddr ValAddress,
	amount Int,
	cfg *TxConfig,
) (*TxResponse, error) {
	if amount.IsNil() || amount.Int64() <= 0 {
		return nil, ErrInvalidAmount
	}

	signer, err := ca.getSigner(cfg)
	if err != nil {
		return nil, err
	}

	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgUndelegate(signer, delAddr, coins)
	return ca.submitMsg(ctx, msg, cfg)
}

func (ca *CoreAccessor) Delegate(
	ctx context.Context,
	delAddr ValAddress,
	amount Int,
	cfg *TxConfig,
) (*TxResponse, error) {
	if amount.IsNil() || amount.Int64() <= 0 {
		return nil, ErrInvalidAmount
	}

	signer, err := ca.getSigner(cfg)
	if err != nil {
		return nil, err
	}

	coins := sdktypes.NewCoin(app.BondDenom, amount)
	msg := stakingtypes.NewMsgDelegate(signer, delAddr, coins)
	return ca.submitMsg(ctx, msg, cfg)
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
	granter, err := ca.getSigner(cfg)
	if err != nil {
		return nil, err
	}

	allowance := &feegrant.BasicAllowance{}
	if !amount.IsZero() {
		// set spend limit
		allowance.SpendLimit = sdktypes.NewCoins(sdktypes.NewCoin(app.BondDenom, amount))
	}

	msg, err := feegrant.NewMsgGrantAllowance(allowance, granter, grantee)
	if err != nil {
		return nil, err
	}
	return ca.submitMsg(ctx, msg, cfg)
}

func (ca *CoreAccessor) RevokeGrantFee(
	ctx context.Context,
	grantee AccAddress,
	cfg *TxConfig,
) (*TxResponse, error) {
	granter, err := ca.getSigner(cfg)
	if err != nil {
		return nil, err
	}

	msg := feegrant.NewMsgRevokeAllowance(granter, grantee)
	return ca.submitMsg(ctx, &msg, cfg)
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

func (ca *CoreAccessor) setupTxClient(ctx context.Context, keyName string) (*user.TxClient, error) {
	encCfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	// explicitly set default address. Otherwise, there could be a mismatch between defaultKey and
	// defaultAddress.
	rec, err := ca.keyring.Key(keyName)
	if err != nil {
		return nil, err
	}
	addr, err := rec.GetAddress()
	if err != nil {
		return nil, err
	}
	ca.defaultSignerAddress = addr
	return user.SetupTxClient(ctx, ca.keyring, ca.coreConn, encCfg,
		user.WithDefaultAccount(keyName), user.WithDefaultAddress(addr),
	)
}

func (ca *CoreAccessor) submitMsg(
	ctx context.Context,
	msg sdktypes.Msg,
	cfg *TxConfig,
) (*TxResponse, error) {
	txConfig := make([]user.TxOption, 0)
	var (
		gas = cfg.GasLimit()
		err error
	)
	if gas == 0 {
		gas, err = estimateGas(ctx, ca.client, msg)
		if err != nil {
			return nil, fmt.Errorf("estimating gas: %w", err)
		}
	}

	gasPrice := cfg.GasPrice()
	if gasPrice == DefaultGasPrice {
		gasPrice = ca.minGasPrice
	}

	txConfig = append(txConfig, user.SetGasLimitAndGasPrice(gas, gasPrice))

	if cfg.FeeGranterAddress() != "" {
		granter, err := parseAccAddressFromString(cfg.FeeGranterAddress())
		if err != nil {
			return nil, fmt.Errorf("getting granter: %w", err)
		}
		txConfig = append(txConfig, user.SetFeeGranter(granter))
	}

	resp, err := ca.client.SubmitTx(ctx, []sdktypes.Msg{msg}, txConfig...)
	return unsetTx(resp), err
}

func (ca *CoreAccessor) getSigner(cfg *TxConfig) (AccAddress, error) {
	switch {
	case cfg.SignerAddress() != "":
		return parseAccAddressFromString(cfg.SignerAddress())
	case cfg.KeyName() != "" && cfg.KeyName() != ca.defaultSignerAccount:
		return parseAccountKey(ca.keyring, cfg.KeyName())
	default:
		return ca.defaultSignerAddress, nil
	}
}

// THIS IS A TEMPORARY SOLUTION!!!
// unsetTx helps to fix issue in TxResponse marshaling. Marshaling TxReponse
// fails because `TxResponse.Tx` is not empty but does not contain respective codec
// for encoding using the standard `json.MarshalJSON()`. It becomes empty when
// https://github.com/celestiaorg/celestia-core/issues/1281 will be merged.
// The `TxResponse.Tx` contains the transaction that is sent to the cosmos-sdk in the form
// in which it is processed there, so the user should not be aware of it.
func unsetTx(txResponse *TxResponse) *TxResponse {
	if txResponse != nil && txResponse.Tx != nil {
		txResponse.Tx = nil
	}
	return txResponse
}
