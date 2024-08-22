package state

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	abci "github.com/tendermint/tendermint/abci/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v2/test/util/genesis"
	"github.com/celestiaorg/celestia-app/v2/test/util/testnode"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
)

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

type IntegrationTestSuite struct {
	suite.Suite

	cleanups []func() error
	accounts []genesis.Account
	cctx     testnode.Context

	accessor *CoreAccessor
}

func (s *IntegrationTestSuite) SetupSuite() {
	if testing.Short() {
		s.T().Skip("skipping test in unit-tests")
	}
	s.T().Log("setting up integration test suite")

	cfg := core.DefaultTestConfig()
	s.cctx = core.StartTestNodeWithConfig(s.T(), cfg)
	s.accounts = cfg.Genesis.Accounts()

	s.Require().Greater(len(s.accounts), 0)
	accountName := s.accounts[0].Name

	accessor, err := NewCoreAccessor(s.cctx.Keyring, accountName, localHeader{s.cctx.Client}, "", "")
	require.NoError(s.T(), err)
	setClients(accessor, s.cctx.GRPCClient)
	s.accessor = accessor

	// required to ensure the Head request is non-nil
	_, err = s.cctx.WaitForHeight(3)
	require.NoError(s.T(), err)
}

func setClients(ca *CoreAccessor, conn *grpc.ClientConn) {
	ca.coreConn = conn
	// create the staking query client
	stakingCli := stakingtypes.NewQueryClient(ca.coreConn)
	ca.stakingCli = stakingCli

	ca.abciQueryCli = tmservice.NewServiceClient(ca.coreConn)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	require := s.Require()
	require.NoError(s.accessor.Stop(s.cctx.GoContext()))
	for _, c := range s.cleanups {
		err := c()
		require.NoError(err)
	}
}

type localHeader struct {
	client rpcclient.Client
}

func (l localHeader) Head(
	ctx context.Context,
	_ ...libhead.HeadOption[*header.ExtendedHeader],
) (*header.ExtendedHeader, error) {
	latest, err := l.client.Block(ctx, nil)
	if err != nil {
		return nil, err
	}
	h := &header.ExtendedHeader{
		RawHeader: latest.Block.Header,
	}
	return h, nil
}

func (s *IntegrationTestSuite) TestGetBalance() {
	require := s.Require()

	for _, account := range s.accounts {
		hexAddress := account.PubKey.Address().String()
		sdkAddress, err := sdk.AccAddressFromHexUnsafe(hexAddress)
		require.NoError(err)

		bal, err := s.accessor.BalanceForAddress(context.Background(), Address{sdkAddress})
		require.NoError(err)
		require.Equal(bal.Denom, appconsts.BondDenom)
		require.True(bal.Amount.GT(sdk.NewInt(1))) // verify that each account has some balance
	}
}

// This test can be used to generate a json encoded block for other test data,
// such as that in share/availability/light/testdata
func (s *IntegrationTestSuite) TestGenerateJSONBlock() {
	t := s.T()
	t.Skip("skipping testdata generation test")
	s.Require().Greater(len(s.accounts), 0)
	accountName := s.accounts[0].Name
	resp, err := s.cctx.FillBlock(4, accountName, flags.BroadcastSync)
	require := s.Require()
	require.NoError(err)
	require.Equal(abci.CodeTypeOK, resp.Code)
	require.NoError(s.cctx.WaitForNextBlock())

	// download the block that the tx was in
	res, err := testnode.QueryWithoutProof(s.cctx.Context, resp.TxHash)
	require.NoError(err)

	block, err := s.cctx.Client.Block(s.cctx.GoContext(), &res.Height)
	require.NoError(err)

	pBlock, err := block.Block.ToProto()
	require.NoError(err)

	file, err := os.OpenFile("sample-block.json", os.O_CREATE|os.O_RDWR, os.ModePerm)
	defer file.Close() //nolint: staticcheck
	require.NoError(err)

	err = json.NewEncoder(file).Encode(pBlock)
	require.NoError(err)
}
