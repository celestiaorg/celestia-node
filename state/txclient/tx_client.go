package txclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	logging "github.com/ipfs/go-log/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v8/app"
	"github.com/celestiaorg/celestia-app/v8/app/encoding"
	apperrors "github.com/celestiaorg/celestia-app/v8/app/errors"
	appfibre "github.com/celestiaorg/celestia-app/v8/fibre"
	"github.com/celestiaorg/celestia-app/v8/pkg/user"
	apptypes "github.com/celestiaorg/celestia-app/v8/x/blob/types"
	libshare "github.com/celestiaorg/go-square/v4/share"
)

var log = logging.Logger("state/txclient")

type TxClient struct {
	ctx    context.Context
	cancel context.CancelFunc

	txClient    *user.TxClient
	fibreClient *appfibre.Client

	keyring              keyring.Keyring
	defaultSignerAccount string
	defaultSignerAddress types.AccAddress
	coreConns            []*grpc.ClientConn
	estimatorServiceAddr string
	estimatorServiceTLS  bool
	estimatorConn        *grpc.ClientConn
	txWorkerAccounts     int

	metrics *metrics

	txClientLk    sync.Mutex
	fibreClientLk sync.Mutex
}

func NewTxClient(
	kr keyring.Keyring,
	defaultSignerAccount string,
	coreConn *grpc.ClientConn,
	opts ...Option,
) (*TxClient, error) {
	rec, err := kr.Key(defaultSignerAccount)
	if err != nil {
		return nil, err
	}
	addr, err := rec.GetAddress()
	if err != nil {
		return nil, err
	}

	client := &TxClient{
		keyring:              kr,
		defaultSignerAddress: addr,
		defaultSignerAccount: defaultSignerAccount,
		coreConns:            []*grpc.ClientConn{coreConn},
	}

	for _, opt := range opts {
		opt(client)
	}
	return client, nil
}

func (c *TxClient) Start(context.Context) error {
	c.ctx, c.cancel = context.WithCancel(context.Background())
	if err := c.setupTxClient(); err != nil {
		log.Warnw("failed to setup tx client", "err", err)
	}
	if err := c.setupFibreClient(); err != nil {
		log.Warnw("failed to setup fibre client", "err", err)
	}
	return nil
}

func setupEstimatorConnection(ctx context.Context, addr string, tlsEnabled bool) (*grpc.ClientConn, error) {
	log.Infow("setting up estimator connection", "address", addr)

	interceptor := grpc_retry.UnaryClientInterceptor(
		grpc_retry.WithMax(5),
		grpc_retry.WithCodes(codes.Unavailable),
		grpc_retry.WithBackoff(
			grpc_retry.BackoffExponentialWithJitter(time.Second, 2.0)),
	)
	grpcOpts := []grpc.DialOption{
		grpc.WithUnaryInterceptor(interceptor),
	}

	if tlsEnabled {
		grpcOpts = append(grpcOpts,
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})),
		)
	} else {
		grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(addr, grpcOpts...)
	if err != nil {
		return nil, fmt.Errorf("state: failed to set up grpc connection to estimator address %s: %w", addr, err)
	}

	conn.Connect()
	if !conn.WaitForStateChange(ctx, connectivity.Ready) {
		return nil, errors.New("couldn't connect to core endpoint")
	}
	return conn, nil
}

func (c *TxClient) Stop(ctx context.Context) error {
	c.cancel()

	if c.fibreClient != nil {
		if err := c.fibreClient.Stop(ctx); err != nil {
			return err
		}
		c.fibreClient = nil
	}

	if c.estimatorConn != nil {
		err := c.estimatorConn.Close()
		if err != nil {
			return err
		}
		c.estimatorConn = nil
	}
	return nil
}

func (c *TxClient) SubmitMessage(
	ctx context.Context,
	msg types.Msg,
	cfg *TxConfig,
) (*user.TxResponse, error) {
	err := c.setupTxClient()
	if err != nil {
		return nil, err
	}

	txConfig := make([]user.TxOption, 0)
	if cfg.FeeGranterAddress() != "" {
		granter, err := ParseAccAddressFromString(cfg.FeeGranterAddress())
		if err != nil {
			return nil, fmt.Errorf("getting granter: %w", err)
		}
		txConfig = append(txConfig, user.SetFeeGranter(granter))
	}

	gasEstimationStart := time.Now()
	gasPrice, gas, err := c.estimateGasPriceAndUsage(ctx, cfg, msg)
	gasEstimationDuration := time.Since(gasEstimationStart)
	c.metrics.observeGasEstimation(ctx, gasEstimationDuration, err)
	c.metrics.observeGasPriceEstimation(ctx, gasEstimationDuration, err)
	if err != nil {
		return nil, err
	}

	txConfig = append(txConfig, user.SetGasLimitAndGasPrice(gas, gasPrice))
	return c.txClient.SubmitTx(ctx, []types.Msg{msg}, txConfig...)
}

func (c *TxClient) SubmitPayForBlob(
	ctx context.Context,
	libBlobs []*libshare.Blob,
	author types.AccAddress,
	cfg *TxConfig,
) (_ *user.TxResponse, err error) {
	if err = c.setupTxClient(); err != nil {
		return nil, err
	}

	start := time.Now()

	// Calculate blob metrics - optimized single pass
	totalSize := int64(0)
	for _, blob := range libBlobs {
		totalSize += int64(len(blob.Data()))
	}

	var gasEstimationDuration time.Duration

	// Use defer to ensure metrics are recorded exactly once at the end
	defer func() {
		c.metrics.observePfbSubmission(ctx, time.Since(start), len(libBlobs), totalSize, gasEstimationDuration, 0, err)
	}()

	ids := make([]string, len(libBlobs))
	dataLengths := make([]int, len(libBlobs))
	for i := range libBlobs {
		if err := libBlobs[i].Namespace().ValidateForBlob(); err != nil {
			return nil, fmt.Errorf("not allowed namespace %s were used to build the blob", libBlobs[i].Namespace().ID())
		}

		ids[i] = libBlobs[i].Namespace().String()
		dataLengths[i] = libBlobs[i].DataLen()
	}

	var feeGrant user.TxOption
	if cfg.FeeGranterAddress() != "" {
		granter, err := ParseAccAddressFromString(cfg.FeeGranterAddress())
		if err != nil {
			return nil, err
		}
		feeGrant = user.SetFeeGranter(granter)
	}

	accountQueryStart := time.Now()
	account := c.txClient.AccountByAddress(ctx, author)
	c.metrics.observeAccountQuery(ctx, time.Since(accountQueryStart), nil)

	if account == nil {
		return nil, fmt.Errorf("account for signer %s not found", author)
	}

	gas := cfg.GasLimit()
	// Gas estimation if needed
	if gas == 0 {
		gasEstimationStart := time.Now()
		gas, err = estimateGasForBlobs(author.String(), libBlobs)
		gasEstimationDuration = time.Since(gasEstimationStart)
		c.metrics.observeGasEstimation(ctx, gasEstimationDuration, err)
		if err != nil {
			return nil, err
		}
	}

	// Gas price estimation
	gasPriceEstimationStart := time.Now()
	gasPrice, err := c.estimateGasPrice(ctx, cfg)
	c.metrics.observeGasPriceEstimation(ctx, time.Since(gasPriceEstimationStart), err)
	if err != nil {
		return nil, err
	}

	opts := []user.TxOption{user.SetGasLimitAndGasPrice(gas, gasPrice)}
	if feeGrant != nil {
		opts = append(opts, feeGrant)
	}
	log.Infow("submitting blobs",
		"amount", len(libBlobs),
		"namespaces", ids,
		"data-lengths", dataLengths,
		"signer", account.Address().String(),
		"key-name", account.Name(),
		"gas-price", gasPrice,
		"gas-limit", gas,
		"fee-granter", cfg.FeeGranterAddress(),
		"tx-priority", int(cfg.TxPriority()),
	)

	defer func() {
		if err != nil {
			log.Errorw("submitting blobs",
				"err", err,
				"namespaces", ids,
				"signer", account.Address().String(),
				"key-name", account.Name(),
			)
		}
	}()

	var response *user.TxResponse
	if c.txWorkerAccounts > 0 && author.Equals(c.defaultSignerAddress) {
		response, err = c.txClient.SubmitPayForBlobToQueue(ctx, libBlobs, opts...)
	} else {
		response, err = c.txClient.SubmitPayForBlobWithAccount(ctx, account.Name(), libBlobs, opts...)
	}

	if apperrors.IsInsufficientFee(err) {
		if cfg.IsGasPriceSet() {
			return nil, fmt.Errorf("failed to submit blobs due to insufficient gas price in txconfig: %w", err)
		}
		return nil, fmt.Errorf("failed to submit blobs due to insufficient estimated "+
			"gas price %f: %w", gasPrice, err)
	}
	return response, err
}

func ParseAccAddressFromString(addrStr string) (types.AccAddress, error) {
	return types.AccAddressFromBech32(addrStr)
}

// estimateGasPriceAndUsage estimate the gas price and limit.
// if gas price and gas limit are both set,
// then these two values will be returned, bypassing other checks.
func (c *TxClient) estimateGasPriceAndUsage(
	ctx context.Context,
	cfg *TxConfig,
	msg types.Msg,
) (float64, uint64, error) {
	if cfg.GasPrice() != DefaultGasPrice && cfg.GasLimit() != 0 {
		return cfg.GasPrice(), cfg.GasLimit(), nil
	}

	gasPrice, gasLimit, err := c.txClient.EstimateGasPriceAndUsage(ctx, []types.Msg{msg}, cfg.TxPriority().ToApp())
	if err != nil {
		return 0, 0, err
	}

	// sanity check estimated gas price against user's set gas price
	if cfg.GasPrice() != DefaultGasPrice {
		gasPrice = cfg.GasPrice()
	}
	if gasPrice > cfg.MaxGasPrice() {
		return 0, 0, ErrGasPriceExceedsLimit
	}

	// sanity check estimated gas usage against user's set gas limit
	if cfg.GasLimit() != 0 {
		gasLimit = cfg.GasLimit()
	}

	return gasPrice, gasLimit, nil
}

// estimateGasForBlobs returns a gas limit that can be applied to the `MsgPayForBlob` transactions.
func estimateGasForBlobs(signer string, blobs []*libshare.Blob) (uint64, error) {
	msg, err := apptypes.NewMsgPayForBlobs(signer, 0, blobs...)
	if err != nil {
		return 0, fmt.Errorf("failed to create msg pay-for-blobs: %w", err)
	}
	return apptypes.DefaultEstimateGas(msg), nil
}

// EstimateGasPrice queries the gas price for a transaction via the estimator
// service, unless user specifies a GasPrice via the TxConfig.
func (c *TxClient) estimateGasPrice(ctx context.Context, cfg *TxConfig) (float64, error) {
	// if user set gas price, use it
	if cfg.GasPrice() != DefaultGasPrice {
		return cfg.GasPrice(), nil
	}

	gasPrice, err := c.txClient.EstimateGasPrice(ctx, cfg.TxPriority().ToApp())
	if err != nil {
		return 0, err
	}

	// sanity check against max gas price
	if gasPrice > cfg.MaxGasPrice() {
		return 0, ErrGasPriceExceedsLimit
	}

	return gasPrice, nil
}

func (c *TxClient) setupTxClient() error {
	c.txClientLk.Lock()
	defer c.txClientLk.Unlock()

	if c.txClient != nil {
		return nil
	}

	opts := []user.Option{user.WithDefaultAddress(c.defaultSignerAddress)}
	if c.estimatorServiceAddr != "" {
		estimatorConn, err := setupEstimatorConnection(c.ctx, c.estimatorServiceAddr, c.estimatorServiceTLS)
		if err != nil {
			return err
		}

		opts = append(opts, user.WithEstimatorService(estimatorConn))
		c.estimatorConn = estimatorConn
	}

	if c.txWorkerAccounts > 1 {
		opts = append(opts, user.WithTxWorkers(c.txWorkerAccounts))
	}

	encCfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)

	if len(c.coreConns) > 1 {
		opts = append(opts, user.WithAdditionalCoreEndpoints(c.coreConns[1:]))
	}

	client, err := user.SetupTxClient(c.ctx, c.keyring, c.coreConns[0], encCfg, opts...)
	if err != nil {
		return fmt.Errorf("failed to setup a tx client: %w", err)
	}

	c.txClient = client
	return nil
}
