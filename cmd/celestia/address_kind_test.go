package main

import (
	"context"
	"testing"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc"
	statecmd "github.com/celestiaorg/celestia-node/nodebuilder/state/cmd"
)

// TestStateCmdWrongAddressKind checks that state subcommands return an error, rather than
// panicking, when given an account address where a validator address is expected (or vice
// versa). parseAddressFromString accepts either kind, and the commands previously used an
// unchecked type assertion to pick out the one they needed.
func TestStateCmdWrongAddressKind(t *testing.T) {
	srv := rpc.NewServer("127.0.0.1", "0", true, rpc.CORSConfig{}, rpc.TLSConfig{}, rpc.RateLimitConfig{}, nil, nil)
	require.NoError(t, srv.Start(context.Background()))
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })
	url := "http://" + srv.ListenAddr()

	t.Cleanup(func() {
		for _, name := range []string{"url", "token"} {
			f := statecmd.Cmd.PersistentFlags().Lookup(name)
			if f == nil {
				continue
			}
			require.NoError(t, f.Value.Set(f.DefValue))
			f.Changed = false
		}
	})

	acc := sdktypes.AccAddress(make([]byte, 20)).String()
	val := sdktypes.ValAddress(make([]byte, 20)).String()

	const wantVal, wantAcc = "expected a validator address", "expected an account address"
	cases := map[string]struct {
		args []string
		want string
	}{
		"withdraw-delegator-reward with account address": {
			[]string{"state", "withdraw-delegator-reward", acc}, wantVal,
		},
		"get-delegation with account address": {[]string{"state", "get-delegation", acc}, wantVal},
		"transfer to validator address":       {[]string{"state", "transfer", val, "100"}, wantAcc},
		"begin-redelegate from account address": {
			[]string{"state", "begin-redelegate", acc, val, "100"}, wantVal,
		},
		"begin-redelegate to account address": {
			[]string{"state", "begin-redelegate", val, acc, "100"}, wantVal,
		},
		"get-redelegations from account address": {
			[]string{"state", "get-redelegations", acc, val}, wantVal,
		},
		"get-redelegations to account address": {
			[]string{"state", "get-redelegations", val, acc}, wantVal,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rootCmd.SetArgs(append(tc.args, "--url", url, "--token", "x"))
			require.NotPanics(t, func() {
				err := rootCmd.ExecuteContext(context.Background())
				require.ErrorContains(t, err, tc.want)
			})
		})
	}
}
