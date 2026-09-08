//go:build fibre_e2e

package fibre

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/cometbft/cometbft/privval"
	core "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v10/app"
	"github.com/celestiaorg/celestia-app/v10/app/encoding"
	appfibre "github.com/celestiaorg/celestia-app/v10/fibre"
	"github.com/celestiaorg/celestia-app/v10/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v10/pkg/user"
	"github.com/celestiaorg/celestia-app/v10/test/util/genesis"
	"github.com/celestiaorg/celestia-app/v10/test/util/testnode"
	fibretypes "github.com/celestiaorg/celestia-app/v10/x/fibre/types"
	valtypes "github.com/celestiaorg/celestia-app/v10/x/valaddr/types"
	libshare "github.com/celestiaorg/go-square/v4/share"

	nodefibre "github.com/celestiaorg/celestia-node/fibre"
	"github.com/celestiaorg/celestia-node/state/txclient"
)

// noEscrowKeyName is funded on chain but never deposits into escrow, so its
// payments must be rejected.
const noEscrowKeyName = "no-escrow-account"

// keep the withdrawal locked for the whole run so the pending-withdrawal
// assertion does not race the module's auto-execution.
const testWithdrawalDelay = time.Hour

func TestFibreE2ESuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fibre e2e test in short mode")
	}
	suite.Run(t, new(FibreE2ESuite))
}

type FibreE2ESuite struct {
	suite.Suite

	cctx   testnode.Context
	server *appfibre.Server
	conn   *grpc.ClientConn

	svc     *nodefibre.Service
	poorSvc *nodefibre.Service // wired to noEscrowKeyName

	appClient, poorAppClient *appfibre.Client
	txClient, poorTxClient   *txclient.TxClient

	signer, poorSigner string
}

func (s *FibreE2ESuite) SetupSuite() {
	t := s.T()

	ecfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	cfg := testnode.DefaultConfig().
		WithFundedAccounts(appfibre.DefaultKeyName, noEscrowKeyName).
		WithDelayedPrecommitTimeout(500 * time.Millisecond).
		WithModifiers(setFibreWithdrawalDelay(ecfg.Codec, testWithdrawalDelay))

	cctx, _, grpcAddr := testnode.NewNetwork(t, cfg)
	s.cctx = cctx

	_, err := cctx.WaitForHeight(1)
	require.NoError(t, err)

	// FSP signs with the validator's file-based priv key.
	pvKeyFile := filepath.Join(cctx.HomeDir, "config", "priv_validator_key.json")
	pvStateFile := filepath.Join(cctx.HomeDir, "data", "priv_validator_state.json")
	filePV := privval.LoadFilePV(pvKeyFile, pvStateFile)

	serverCfg := appfibre.DefaultServerConfig()
	serverCfg.AppGRPCAddress = grpcAddr
	serverCfg.ServerListenAddress = "127.0.0.1:0"
	serverCfg.SignerFn = func(string) (core.PrivValidator, error) { return filePV, nil }
	serverCfg.StoreFn = func(sc appfibre.StoreConfig) (*appfibre.Store, error) {
		return appfibre.NewMemoryStore(sc), nil
	}
	s.server, err = appfibre.NewServer(serverCfg)
	require.NoError(t, err)
	require.NoError(t, s.server.Start(cctx.GoContext()))

	// register the FSP on chain so the client's default host registry resolves it.
	s.registerHost(s.server.ListenAddress())

	s.conn, err = grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	s.txClient, s.appClient, s.svc = s.newService(appfibre.DefaultKeyName, grpcAddr)
	s.poorTxClient, s.poorAppClient, s.poorSvc = s.newService(noEscrowKeyName, grpcAddr)

	s.signer = s.address(appfibre.DefaultKeyName)
	s.poorSigner = s.address(noEscrowKeyName)
}

func (s *FibreE2ESuite) address(keyName string) string {
	keyInfo, err := s.cctx.Keyring.Key(keyName)
	require.NoError(s.T(), err)
	addr, err := keyInfo.GetAddress()
	require.NoError(s.T(), err)
	return addr.String()
}

// newService builds a node-side Fibre stack (tx client + appfibre client via the
// default on-chain host-discovery path + Service) bound to the given key.
func (s *FibreE2ESuite) newService(keyName, grpcAddr string) (*txclient.TxClient, *appfibre.Client, *nodefibre.Service) {
	t := s.T()

	tc, err := txclient.NewTxClient(s.cctx.Keyring, keyName, s.conn)
	require.NoError(t, err)
	require.NoError(t, tc.Start(s.cctx.GoContext()))

	clientCfg := appfibre.DefaultClientConfig()
	clientCfg.DefaultKeyName = keyName
	clientCfg.StateAddress = grpcAddr
	appClient, err := appfibre.NewClient(s.cctx.Keyring, clientCfg)
	require.NoError(t, err)
	require.NoError(t, appClient.Start(s.cctx.GoContext()))

	acc := nodefibre.NewAccountClient(tc, s.conn)
	return tc, appClient, nodefibre.NewService(appClient, tc, acc)
}

func (s *FibreE2ESuite) TearDownSuite() {
	ctx := context.Background()
	for _, c := range []*appfibre.Client{s.appClient, s.poorAppClient} {
		if c != nil {
			_ = c.Stop(ctx)
		}
	}
	if s.server != nil {
		_ = s.server.Stop(ctx)
	}
	for _, tc := range []*txclient.TxClient{s.txClient, s.poorTxClient} {
		if tc != nil {
			_ = tc.Stop(ctx)
		}
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

// registerHost publishes the FSP's listen address for the single validator,
// mirroring the on-chain registration a real FSP performs.
func (s *FibreE2ESuite) registerHost(host string) {
	t := s.T()
	ctx := s.cctx.GoContext()

	stakingClient := stakingtypes.NewQueryClient(s.cctx.GRPCClient)
	vals, err := stakingClient.Validators(ctx, &stakingtypes.QueryValidatorsRequest{})
	require.NoError(t, err)
	require.Len(t, vals.Validators, 1)

	txClient, err := testnode.NewTxClientFromContext(s.cctx)
	require.NoError(t, err)

	msg := &valtypes.MsgSetFibreProviderInfo{Signer: vals.Validators[0].OperatorAddress, Host: host}
	resp, err := txClient.SubmitTx(ctx, []sdk.Msg{msg}, user.SetGasLimit(200_000), user.SetFee(5_000))
	require.NoError(t, err)
	require.Equal(t, uint32(0), resp.Code)
	require.NoError(t, s.cctx.WaitForNextBlock())

	tmClient := cmtservice.NewServiceClient(s.cctx.GRPCClient)
	valSet, err := tmClient.GetLatestValidatorSet(ctx, &cmtservice.GetLatestValidatorSetRequest{})
	require.NoError(t, err)
	consAddr, err := sdk.ConsAddressFromBech32(valSet.Validators[0].Address)
	require.NoError(t, err)

	info, err := valtypes.NewQueryClient(s.cctx.GRPCClient).FibreProviderInfo(ctx,
		&valtypes.QueryFibreProviderInfoRequest{ValidatorConsensusAddress: consAddr.String()})
	require.NoError(t, err)
	require.True(t, info.Found)
	require.Equal(t, host, info.Info.Host)
}

func (s *FibreE2ESuite) TestDepositAndQuery() {
	t := s.T()
	ctx := s.cctx.GoContext()

	deposit := sdk.NewCoin(appconsts.BondDenom, sdkmath.NewInt(50_000_000))
	require.NoError(t, s.svc.Deposit(ctx, deposit, txclient.NewTxConfig()))
	require.NoError(t, s.cctx.WaitForNextBlock())

	acc, err := s.svc.QueryEscrowAccount(ctx, s.signer)
	require.NoError(t, err)
	require.Equal(t, s.signer, acc.Signer)
	require.Equal(t, deposit, acc.Balance)
	require.Equal(t, deposit, acc.AvailableBalance)
}

func (s *FibreE2ESuite) TestSubmit() {
	t := s.T()
	ctx := s.cctx.GoContext()
	require.NoError(t, s.cctx.WaitForNextBlock())

	before, err := s.svc.QueryEscrowAccount(ctx, s.signer)
	require.NoError(t, err)

	data := randomBytes(t, 4*1024)
	ns := libshare.MustNewV0Namespace([]byte{0xDE, 0xAD})

	resp, promise, err := s.svc.Submit(ctx, ns, data, txclient.NewTxConfig())
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotZero(t, resp.Height)
	require.NotEmpty(t, resp.TxHash)
	require.NotEmpty(t, promise.ValidatorSignatures)

	// escrow drops by exactly the payment for the padded upload size.
	uploadSize := uint32(appfibre.DefaultBlobConfigV0().UploadSize(len(data)))
	wantDebit := fibretypes.PaymentAmount(uploadSize)
	after, err := s.svc.QueryEscrowAccount(ctx, s.signer)
	require.NoError(t, err)
	require.Equal(t, wantDebit, before.Balance.Sub(after.Balance))
	require.Equal(t, wantDebit, before.AvailableBalance.Sub(after.AvailableBalance))
}

func (s *FibreE2ESuite) TestUploadAndDownload() {
	t := s.T()
	ctx := s.cctx.GoContext()
	require.NoError(t, s.cctx.WaitForNextBlock())

	data := randomBytes(t, 32*1024)
	ns := libshare.MustNewV0Namespace([]byte{0xCA, 0xFE})

	promise, blobID, err := s.svc.Upload(ctx, ns, data, txclient.NewTxConfig())
	require.NoError(t, err)
	require.NotEmpty(t, promise.ValidatorSignatures)
	require.NoError(t, blobID.Validate())

	got, err := s.svc.Download(ctx, blobID)
	require.NoError(t, err)
	require.Equal(t, data, got)
}

func (s *FibreE2ESuite) TestWithdrawAndPending() {
	t := s.T()
	ctx := s.cctx.GoContext()
	require.NoError(t, s.cctx.WaitForNextBlock())

	withdraw := sdk.NewCoin(appconsts.BondDenom, sdkmath.NewInt(5_000_000))
	require.NoError(t, s.svc.Withdraw(ctx, withdraw, txclient.NewTxConfig()))
	require.NoError(t, s.cctx.WaitForNextBlock())

	pending, err := s.svc.PendingWithdrawals(ctx, s.signer)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.Equal(t, s.signer, pending[0].Signer)
	require.Equal(t, withdraw, pending[0].Amount)
}

func (s *FibreE2ESuite) TestSubmitInsufficientEscrow() {
	t := s.T()
	ctx := s.cctx.GoContext()
	require.NoError(t, s.cctx.WaitForNextBlock())

	data := randomBytes(t, 4*1024)
	ns := libshare.MustNewV0Namespace([]byte{0x1D, 0x1E})

	_, _, err := s.poorSvc.Submit(ctx, ns, data, txclient.NewTxConfig())
	require.Error(t, err, "submit must fail when the signer has no escrow")

	_, err = s.poorSvc.QueryEscrowAccount(ctx, s.poorSigner)
	require.Error(t, err, "no escrow account should exist after a rejected submit")
}

func (s *FibreE2ESuite) TestDownloadFailures() {
	ctx := s.cctx.GoContext()

	s.Run("NotFound", func() {
		t := s.T()
		var c appfibre.Commitment
		_, err := rand.Read(c[:])
		require.NoError(t, err)

		_, err = s.svc.Download(ctx, appfibre.NewBlobID(0, c))
		require.ErrorIs(t, err, appfibre.ErrNotFound)
	})

	s.Run("MalformedID", func() {
		t := s.T()
		_, err := s.svc.Download(ctx, appfibre.BlobID{0x00})
		require.ErrorContains(t, err, "blob ID")
	})
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

func setFibreWithdrawalDelay(cdc codec.Codec, delay time.Duration) genesis.Modifier {
	return func(state map[string]json.RawMessage) map[string]json.RawMessage {
		gs := fibretypes.DefaultGenesis()
		gs.Params.WithdrawalDelay = delay
		state[fibretypes.ModuleName] = cdc.MustMarshalJSON(gs)
		return state
	}
}
