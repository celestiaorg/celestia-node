package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc"
	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// nodeStub records how the token requests it serves were made.
type nodeStub struct {
	called string
	perms  []auth.Permission
	ttl    time.Duration
}

func (n *nodeStub) Info(context.Context) (node.Info, error) { return node.Info{}, nil }

func (n *nodeStub) Ready(context.Context) (bool, error) { return true, nil }

func (n *nodeStub) LogLevelSet(context.Context, string, string) error { return nil }

func (n *nodeStub) AuthVerify(context.Context, string) ([]auth.Permission, error) { return nil, nil }

func (n *nodeStub) AuthNew(context.Context, []auth.Permission) (string, error) {
	n.called = "AuthNew"
	return "without-ttl", nil
}

func (n *nodeStub) AuthNewWithExpiry(_ context.Context, perms []auth.Permission, ttl time.Duration) (string, error) {
	n.called = "AuthNewWithExpiry"
	n.perms = perms
	n.ttl = ttl
	return "with-ttl", nil
}

// newNodeRPC serves the given stub over RPC, and points the command at it the
// same way the celestia binary wires its RPC flags.
func newNodeRPC(t *testing.T, stub *nodeStub) *rpc.Server {
	t.Helper()

	srv := rpc.NewServer("127.0.0.1", "0", true, rpc.CORSConfig{}, rpc.TLSConfig{}, rpc.RateLimitConfig{}, nil, nil)
	srv.RegisterService("node", stub, &node.API{})
	require.NoError(t, srv.Start(context.Background()))
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })

	Cmd.PersistentFlags().AddFlagSet(cmdnode.RPCFlags())
	Cmd.SilenceErrors, Cmd.SilenceUsage = true, true
	t.Cleanup(func() {
		// don't leave the flags set for tests that reuse the command
		for _, f := range []*pflag.Flag{
			Cmd.PersistentFlags().Lookup("url"),
			Cmd.PersistentFlags().Lookup("token"),
			authCmd.Flags().Lookup(ttlFlagName),
		} {
			require.NotNil(t, f)
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})

	return srv
}

func TestSetPermissionsWithTTL(t *testing.T) {
	stub := &nodeStub{}
	srv := newNodeRPC(t, stub)

	Cmd.SetArgs([]string{
		"set-permissions", "admin",
		"--ttl", "1h",
		"--url", "http://" + srv.ListenAddr(), "--token", "test",
	})
	require.NoError(t, Cmd.ExecuteContext(context.Background()))

	require.Equal(t, "AuthNewWithExpiry", stub.called)
	require.Equal(t, time.Hour, stub.ttl)
	require.Equal(t, []auth.Permission{"admin"}, stub.perms)
}

func TestSetPermissionsWithoutTTL(t *testing.T) {
	stub := &nodeStub{}
	srv := newNodeRPC(t, stub)

	Cmd.SetArgs([]string{
		"set-permissions", "admin",
		"--url", "http://" + srv.ListenAddr(), "--token", "test",
	})
	require.NoError(t, Cmd.ExecuteContext(context.Background()))

	require.Equal(t, "AuthNew", stub.called)
	require.Zero(t, stub.ttl)
}
