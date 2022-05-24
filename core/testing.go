package core

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	tmservice "github.com/tendermint/tendermint/libs/service"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	tmtypes "github.com/tendermint/tendermint/types"
)

// so that we never hit an issue where we request blocks that are removed
const defaultRetainBlocks int64 = 10000

// StartTestNode starts a mock Core node background process and returns it.
func StartTestNode(ctx context.Context, t *testing.T, app types.Application, cfg *config.Config) tmservice.Service {
	nd, closer, err := rpctest.StartTendermint(ctx, cfg, app, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, closer(ctx))
	})
	return nd
}

// StartTestKVApp starts Tendermint KVApp.
func StartTestKVApp(ctx context.Context, t *testing.T) (tmservice.Service, types.Application, *config.Config) {
	cfg, err := rpctest.CreateConfig("Dummy_TmNode")
	require.NoError(t, err)
	app := CreateKVStore(defaultRetainBlocks)
	return StartTestNode(ctx, t, app, cfg), app, cfg
}

// CreateKVStore creates a simple kv store app and gives the user
// ability to set desired amount of blocks to be retained.
func CreateKVStore(retainBlocks int64) *kvstore.Application {
	app := kvstore.NewApplication()
	app.RetainBlocks = retainBlocks
	return app
}

// StartTestClient returns a started remote Core node process, as well its
// mock Core Client.
func StartTestClient(ctx context.Context, t *testing.T) (tmservice.Service, Client) {
	nd, _, cfg := StartTestKVApp(ctx, t)
	protocol, ip := GetEndpoint(cfg)
	client, err := NewRemote(protocol, ip)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := client.Stop()
		require.NoError(t, err)
	})
	err = client.Start()
	require.NoError(t, err)
	return nd, client
}

// GetEndpoint returns the protocol and ip of the remote node.
func GetEndpoint(cfg *config.Config) (string, string) {
	endpoint := cfg.RPC.ListenAddress
	protocol, ip := endpoint[:3], endpoint[6:]
	return protocol, ip
}

func RandValidator(randPower bool, minPower int64) (*tmtypes.Validator, tmtypes.PrivValidator) {
	privVal := tmtypes.NewMockPV()
	votePower := minPower
	if randPower {
		// nolint:gosec // G404: Use of weak random number generator
		votePower += int64(rand.Uint32())
	}
	pubKey, err := privVal.GetPubKey(context.Background())
	if err != nil {
		panic(fmt.Errorf("could not retrieve pubkey %w", err))
	}
	val := tmtypes.NewValidator(pubKey, votePower)
	return val, privVal
}

func RandValidatorSet(numValidators int, votingPower int64) (*tmtypes.ValidatorSet, []tmtypes.PrivValidator) {
	var (
		valz           = make([]*tmtypes.Validator, numValidators)
		privValidators = make([]tmtypes.PrivValidator, numValidators)
	)

	for i := 0; i < numValidators; i++ {
		val, privValidator := RandValidator(false, votingPower)
		valz[i] = val
		privValidators[i] = privValidator
	}

	sort.Sort(tmtypes.PrivValidatorsByAddress(privValidators))

	return tmtypes.NewValidatorSet(valz), privValidators
}

func MakeCommit(blockID tmtypes.BlockID, height int64, round int32,
	voteSet *tmtypes.VoteSet, validators []tmtypes.PrivValidator, now time.Time) (*tmtypes.Commit, error) {

	// all sign
	for i := 0; i < len(validators); i++ {
		pubKey, err := validators[i].GetPubKey(context.Background())
		if err != nil {
			return nil, fmt.Errorf("can't get pubkey: %w", err)
		}
		vote := &tmtypes.Vote{
			ValidatorAddress: pubKey.Address(),
			ValidatorIndex:   int32(i),
			Height:           height,
			Round:            round,
			Type:             tmproto.PrecommitType,
			BlockID:          blockID,
			Timestamp:        now,
		}

		_, err = signAddVote(validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}

func signAddVote(privVal tmtypes.PrivValidator, vote *tmtypes.Vote, voteSet *tmtypes.VoteSet) (signed bool, err error) {
	v := vote.ToProto()
	err = privVal.SignVote(context.Background(), voteSet.ChainID(), v)
	if err != nil {
		return false, err
	}
	vote.Signature = v.Signature
	return voteSet.AddVote(vote)
}
