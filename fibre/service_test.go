package fibre

import (
	"context"
	"crypto/ed25519"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	core "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v10/app"
	"github.com/celestiaorg/celestia-app/v10/app/encoding"
	appfibre "github.com/celestiaorg/celestia-app/v10/fibre"
	appstate "github.com/celestiaorg/celestia-app/v10/fibre/state"
	"github.com/celestiaorg/celestia-app/v10/fibre/validator"
	fibretypes "github.com/celestiaorg/celestia-app/v10/x/fibre/types"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/state/txclient"
)

const testKeyName = "svc-test"

// fixedSetStateClient is a minimal appstate.Client backed by a fixed validator set.
type fixedSetStateClient struct{ set validator.Set }

func (s *fixedSetStateClient) Head(context.Context) (validator.Set, error) { return s.set, nil }
func (s *fixedSetStateClient) GetByHeight(context.Context, uint64) (validator.Set, error) {
	return s.set, nil
}

func (s *fixedSetStateClient) GetHost(context.Context, *core.Validator) (validator.Host, error) {
	return "unused:1", nil
}
func (s *fixedSetStateClient) ChainID() string { return "test-chain" }
func (s *fixedSetStateClient) VerifyPromise(
	context.Context, *appstate.PaymentPromise,
) (appstate.VerifiedPromise, error) {
	return appstate.VerifiedPromise{}, nil
}

func (s *fixedSetStateClient) FullStakeStorageBudget(context.Context) (int64, error) { return 0, nil }
func (s *fixedSetStateClient) Start(context.Context) error                           { return nil }
func (s *fixedSetStateClient) Stop(context.Context) error                            { return nil }

// fakeFSP is an in-process fibre shard provider. A slow FSP takes `delay` to
// accept a shard and honors ctx cancellation like a real gRPC call would.
type fakeFSP struct {
	priv     cmted25519.PrivKey
	delay    time.Duration
	received *atomic.Int64
	canceled *atomic.Int64
}

func (v *fakeFSP) UploadShard(
	ctx context.Context, req *fibretypes.UploadShardRequest, _ ...grpc.CallOption,
) (*fibretypes.UploadShardResponse, error) {
	if v.delay > 0 {
		select {
		case <-time.After(v.delay):
		case <-ctx.Done():
			v.canceled.Add(1)
			return nil, ctx.Err()
		}
	}
	var pp appfibre.PaymentPromise
	if err := pp.FromProto(req.Promise); err != nil {
		return nil, err
	}
	sb, err := pp.SignBytes()
	if err != nil {
		return nil, err
	}
	v.received.Add(1)
	return &fibretypes.UploadShardResponse{
		ValidatorSignature: ed25519.Sign(ed25519.PrivateKey(v.priv.Bytes()), sb),
	}, nil
}

func (v *fakeFSP) DownloadShard(
	context.Context, *fibretypes.DownloadShardRequest, ...grpc.CallOption,
) (*fibretypes.DownloadShardResponse, error) {
	return &fibretypes.DownloadShardResponse{}, nil
}
func (v *fakeFSP) Close() error { return nil }

// newTestUploadService wires a real appfibre.Client (with fake in-process FSPs
// injected via ClientConfig.NewClientFn, whose type lives in an internal
// package, hence reflect) into a real *Service, exactly like nodebuilder/fibre
// does for a live node.
func newTestUploadService(
	t *testing.T, fast, slow int, slowDelay time.Duration,
) (svc *Service, received, canceled *atomic.Int64) {
	t.Helper()
	encCfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	kr := keyring.NewInMemory(encCfg.Codec)
	_, _, err := kr.NewMnemonic(testKeyName, keyring.English, "m/44'/118'/0'/0/0",
		keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	require.NoError(t, err)

	received, canceled = new(atomic.Int64), new(atomic.Int64)
	n := fast + slow
	vals := make([]*core.Validator, n)
	fsps := make(map[string]*fakeFSP, n)
	for i := range n {
		priv := cmted25519.GenPrivKey()
		vals[i] = &core.Validator{Address: priv.PubKey().Address(), PubKey: priv.PubKey(), VotingPower: 100}
		v := &fakeFSP{priv: priv, received: received, canceled: canceled}
		if i >= fast {
			v.delay = slowDelay
		}
		fsps[vals[i].Address.String()] = v
	}
	set := validator.Set{ValidatorSet: core.NewValidatorSet(vals), Height: 10}

	cfg := appfibre.DefaultClientConfig()
	cfg.DefaultKeyName = testKeyName
	cfg.StateClientFn = func() (appstate.Client, error) { return &fixedSetStateClient{set: set}, nil }

	// NewClientFn's type lives in celestia-app's internal fibre gRPC package,
	// so it can't be referenced directly; set it through reflection instead.
	fnField := reflect.ValueOf(&cfg).Elem().FieldByName("NewClientFn")
	fnType := fnField.Type()
	fnField.Set(reflect.MakeFunc(fnType, func(args []reflect.Value) []reflect.Value {
		val := args[1].Interface().(*core.Validator)
		return []reflect.Value{
			reflect.ValueOf(fsps[val.Address.String()]).Convert(fnType.Out(0)),
			reflect.Zero(fnType.Out(1)),
		}
	}))

	fc, err := appfibre.NewClient(kr, cfg)
	require.NoError(t, err)
	require.NoError(t, fc.Start(context.Background()))
	t.Cleanup(func() { _ = fc.Stop(context.Background()) })

	conn, err := grpc.NewClient("passthrough:///127.0.0.1:1",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	tc, err := txclient.NewTxClient(kr, testKeyName, conn)
	require.NoError(t, err)
	require.NoError(t, tc.Start(context.Background()))

	return NewService(fc, tc, NewAccountClient(tc, conn)), received, canceled
}

// TestUploadSurvivesCallerCtxCancelAfterQuorum checks that shards still being sent after
// quorum are not dropped when the caller cancels ctx right after Upload returns.
func TestUploadSurvivesCallerCtxCancelAfterQuorum(t *testing.T) {
	const fast, slow = 7, 3 // 70% fast => quorum reached without the slow 30%
	svc, received, canceled := newTestUploadService(t, fast, slow, 300*time.Millisecond)

	ns := libshare.MustNewV0Namespace([]byte("svc-test"))
	ctx, cancel := context.WithCancel(context.Background())
	_, _, err := svc.Upload(ctx, ns, make([]byte, 1024), nil)
	require.NoError(t, err)
	cancel() // what the JSON-RPC server does the instant the handler returns

	svc.fibreClient.Await() // wait for the background fan-out to finish
	t.Logf("validators that stored their shard: %d/%d, background uploads canceled: %d",
		received.Load(), fast+slow, canceled.Load())
	require.EqualValues(t, fast+slow, received.Load(),
		"post-quorum shards must not be dropped when the caller cancels ctx right after Upload returns")
}
