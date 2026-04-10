# Remove go-fraud Dependency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the `github.com/celestiaorg/go-fraud` dependency and all fraud proof related dead code from celestia-node, since fraud proofs are no longer functional post-shwap and there are no plans to reintroduce them.

**Architecture:** This is a pure code removal. The fraud system has three integration layers: (1) the `nodebuilder/fraud` package providing the fraud service, (2) `ServiceBreaker` wrappers around DASer/Syncer/CoreAccessor that stop services on fraud detection, and (3) BEFP (Bad Encoding Fraud Proof) creation/broadcast in the DAS sampling path. All three layers are removed. The `ErrByzantine` error type and `ShareWithProof` type in `share/eds/byzantine/` are preserved because they're still used by the cascade getter and full availability checker for error classification.

**Tech Stack:** Go, uber/fx dependency injection, go-fraud library (being removed)

**Issue:** Closes https://github.com/celestiaorg/celestia-node/issues/4930

---

### Task 1: Delete the `nodebuilder/fraud/` Package

**Files:**
- Delete: `nodebuilder/fraud/fraud.go`
- Delete: `nodebuilder/fraud/constructors.go`
- Delete: `nodebuilder/fraud/lifecycle.go`
- Delete: `nodebuilder/fraud/module.go`
- Delete: `nodebuilder/fraud/unmarshaler.go`
- Delete: `nodebuilder/fraud/mocks/api.go`

This package contains the fraud Module interface, API struct, ServiceBreaker, constructors, and proof unmarshaler. All of it is dead code without fraud proof support.

- [ ] **Step 1: Delete the fraud package directory**

```bash
rm -rf nodebuilder/fraud/
```

- [ ] **Step 2: Verify deletion**

```bash
ls nodebuilder/fraud/ 2>&1
```

Expected: `No such file or directory`

- [ ] **Step 3: Commit**

```bash
git add -A nodebuilder/fraud/
git commit -m "refactor: delete nodebuilder/fraud package

Remove the entire fraud module package including the Module interface,
API struct, ServiceBreaker, constructors, and proof unmarshaler.
Fraud proofs are no longer functional post-shwap."
```

---

### Task 2: Delete Fraud Test Files and Test Helpers

**Files:**
- Delete: `nodebuilder/tests/fraud_test.go`
- Delete: `header/headertest/fraud/testing.go`

The fraud integration test was already skipped (`t.Skip("unsupported temporary")`). The `FraudMaker` test helper is only used by this test.

- [ ] **Step 1: Delete fraud test file**

```bash
rm nodebuilder/tests/fraud_test.go
```

- [ ] **Step 2: Delete FraudMaker test helper**

```bash
rm -rf header/headertest/fraud/
```

- [ ] **Step 3: Verify deletion**

```bash
ls nodebuilder/tests/fraud_test.go header/headertest/fraud/ 2>&1
```

Expected: Both files not found.

- [ ] **Step 4: Commit**

```bash
git add nodebuilder/tests/fraud_test.go header/headertest/fraud/
git commit -m "refactor: delete fraud test files and FraudMaker helper

Remove the skipped fraud integration test and the FraudMaker
test helper that was only used by the fraud test."
```

---

### Task 3: Delete BEFP Implementation and Proto Files

**Files:**
- Delete: `share/eds/byzantine/bad_encoding.go`
- Delete: `share/eds/byzantine/bad_encoding_test.go`
- Delete: `share/eds/byzantine/pb/share.pb.go`
- Delete: `share/eds/byzantine/pb/share.proto`

The `BadEncodingProof` type implements `fraud.Proof` and is the only fraud proof type registered. The protobuf definitions are used exclusively for BEFP serialization.

**Important:** Do NOT delete `share/eds/byzantine/byzantine.go` or `share/eds/byzantine/share_proof.go` as `ErrByzantine` and `ShareWithProof` are still used by `share/shwap/getters/cascade.go` and `share/availability/full/availability.go`.

- [ ] **Step 1: Delete BEFP implementation and tests**

```bash
rm share/eds/byzantine/bad_encoding.go
rm share/eds/byzantine/bad_encoding_test.go
```

- [ ] **Step 2: Delete proto files**

```bash
rm -rf share/eds/byzantine/pb/
```

- [ ] **Step 3: Verify deletion**

```bash
ls share/eds/byzantine/bad_encoding.go share/eds/byzantine/bad_encoding_test.go share/eds/byzantine/pb/ 2>&1
```

Expected: All files not found.

- [ ] **Step 4: Commit**

```bash
git add share/eds/byzantine/bad_encoding.go share/eds/byzantine/bad_encoding_test.go share/eds/byzantine/pb/
git commit -m "refactor: delete BadEncodingProof implementation and proto files

Remove the BEFP (Bad Encoding Fraud Proof) implementation and its
protobuf definitions. ErrByzantine and ShareWithProof are preserved
as they are still used for error classification."
```

---

### Task 4: Clean Up `share/eds/byzantine/share_proof.go`

**Files:**
- Modify: `share/eds/byzantine/share_proof.go`

Remove proto-related functions that were only used by `bad_encoding.go`: `ShareWithProofToProto()`, `ProtoToShare()`, `ProtoToProof()`. Remove the `pb` and `nmt_pb` imports.

- [ ] **Step 1: Remove unused proto imports and functions from share_proof.go**

Remove the `pb` and `nmt_pb` imports, and delete the following functions:
- `ShareWithProofToProto()` (lines 56-71)
- `ProtoToShare()` (lines 117-135)
- `ProtoToProof()` (lines 137-143)

The remaining file should contain:

```go
package byzantine

import (
	"context"
	"errors"

	"github.com/ipfs/boxo/blockservice"
	logging "github.com/ipfs/go-log/v2"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var log = logging.Logger("share/byzantine")

// ShareWithProof contains data with corresponding Merkle Proof
type ShareWithProof struct {
	// Share is a full data including namespace
	libshare.Share
	// Proof is a Merkle Proof of current share
	Proof *nmt.Proof
	// Axis is a proof axis
	Axis rsmt2d.Axis
}

// Validate validates inclusion of the share under the given root CID.
func (s *ShareWithProof) Validate(roots *share.AxisRoots, axisType rsmt2d.Axis, axisIdx, shrIdx int) bool {
	var rootHash []byte
	switch axisType {
	case rsmt2d.Row:
		rootHash = rootHashForCoordinates(roots, s.Axis, shrIdx, axisIdx)
	case rsmt2d.Col:
		rootHash = rootHashForCoordinates(roots, s.Axis, axisIdx, shrIdx)
	}

	edsSize := len(roots.RowRoots)
	isParity := shrIdx >= edsSize/2 || axisIdx >= edsSize/2
	namespace := libshare.ParitySharesNamespace
	if !isParity {
		namespace = s.Namespace()
	}
	return s.Proof.VerifyInclusion(
		share.NewSHA256Hasher(),
		namespace.Bytes(),
		[][]byte{s.ToBytes()},
		rootHash,
	)
}

// GetShareWithProof attempts to get a share with proof for the given share. It first tries to get
// a row proof and if that fails or proof is invalid, it tries to get a column proof.
func GetShareWithProof(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	roots *share.AxisRoots,
	share libshare.Share,
	axisType rsmt2d.Axis, axisIdx, shrIdx int,
) (*ShareWithProof, error) {
	if axisType == rsmt2d.Col {
		axisIdx, shrIdx, axisType = shrIdx, axisIdx, rsmt2d.Row
	}
	width := len(roots.RowRoots)
	// try row proofs
	root := roots.RowRoots[axisIdx]
	proof, err := ipld.GetProof(ctx, bGetter, root, shrIdx, width)
	if err == nil {
		shareWithProof := &ShareWithProof{
			Share: share,
			Proof: &proof,
			Axis:  rsmt2d.Row,
		}
		if shareWithProof.Validate(roots, axisType, axisIdx, shrIdx) {
			return shareWithProof, nil
		}
	}

	// try column proofs
	root = roots.ColumnRoots[shrIdx]
	proof, err = ipld.GetProof(ctx, bGetter, root, axisIdx, width)
	if err != nil {
		return nil, err
	}
	shareWithProof := &ShareWithProof{
		Share: share,
		Proof: &proof,
		Axis:  rsmt2d.Col,
	}
	if shareWithProof.Validate(roots, axisType, axisIdx, shrIdx) {
		return shareWithProof, nil
	}
	return nil, errors.New("failed to collect proof")
}

func rootHashForCoordinates(r *share.AxisRoots, axisType rsmt2d.Axis, x, y int) []byte {
	if axisType == rsmt2d.Row {
		return r.RowRoots[y]
	}
	return r.ColumnRoots[x]
}
```

- [ ] **Step 2: Verify the file compiles**

```bash
go build ./share/eds/byzantine/...
```

Expected: success (no errors)

- [ ] **Step 3: Run remaining byzantine tests**

Note: There are no remaining test files in this package after deleting `bad_encoding_test.go`, so just verify the package builds.

```bash
go vet ./share/eds/byzantine/...
```

Expected: success

- [ ] **Step 4: Commit**

```bash
git add share/eds/byzantine/share_proof.go
git commit -m "refactor: remove unused proto serialization from share_proof.go

Remove ShareWithProofToProto, ProtoToShare, and ProtoToProof which
were only used by the deleted BadEncodingProof."
```

---

### Task 5: Remove Fraud from DASer (`das/daser.go`)

**Files:**
- Modify: `das/daser.go`

Remove the `fraud.Broadcaster` field, the `go-fraud` import, the `byzantine` import, and the BEFP broadcast logic in the `sample()` method. Remove the `bcast` parameter from `NewDASer`.

- [ ] **Step 1: Write the failing test — verify DASer compiles without fraud**

Before modifying, run the current tests to establish baseline:

```bash
go test ./das/ -run TestDASerLifecycle -count=1 -v
```

Expected: PASS (establishes the tests work before our changes)

- [ ] **Step 2: Modify `das/daser.go`**

Remove `fraud` and `byzantine` imports. Remove the `bcast` field from the `DASer` struct. Remove the `bcast` parameter from `NewDASer`. Simplify the `sample()` method to just return the error without broadcasting.

The `DASer` struct should become:

```go
type DASer struct {
	params Parameters

	da     share.Availability
	hsub   libhead.Subscriber[*header.ExtendedHeader] // listens for new headers in the network
	getter libhead.Store[*header.ExtendedHeader]      // retrieves past headers

	sampler    *samplingCoordinator
	store      checkpointStore
	subscriber subscriber

	cancel         context.CancelFunc
	subscriberDone chan struct{}
	running        atomic.Bool
}
```

The `NewDASer` function signature should become:

```go
func NewDASer(
	da share.Availability,
	hsub libhead.Subscriber[*header.ExtendedHeader],
	getter libhead.Store[*header.ExtendedHeader],
	dstore datastore.Datastore,
	shrexBroadcast shrexsub.BroadcastFn,
	options ...Option,
) (*DASer, error) {
```

Note: The `bcast fraud.Broadcaster[*header.ExtendedHeader]` parameter is removed entirely (it was between `dstore` and `shrexBroadcast`).

The `NewDASer` body should no longer set `bcast`:

```go
	d := &DASer{
		params:         DefaultParameters(),
		da:             da,
		hsub:           hsub,
		getter:         getter,
		store:          newCheckpointStore(dstore),
		subscriber:     newSubscriber(),
		subscriberDone: make(chan struct{}),
	}
```

The `sample()` method should become:

```go
func (d *DASer) sample(ctx context.Context, h *header.ExtendedHeader) error {
	return d.da.SharesAvailable(ctx, h)
}
```

The imports should become:

```go
import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
)
```

- [ ] **Step 3: Verify the file compiles**

```bash
go build ./das/...
```

Expected: Compilation errors in `das/daser_test.go` (test calls `NewDASer` with wrong number of args). This is expected — we fix tests in the next step.

- [ ] **Step 4: Commit**

```bash
git add das/daser.go
git commit -m "refactor: remove fraud broadcaster from DASer

Remove the fraud.Broadcaster field, parameter, and BEFP broadcast
logic from the DASer. Byzantine errors are still detected and
returned but no longer broadcast as fraud proofs."
```

---

### Task 6: Update DASer Tests (`das/daser_test.go`)

**Files:**
- Modify: `das/daser_test.go`

Remove the `fraudtest` import. Update `createDASerSubcomponents` to no longer return a fraud service. Update all `NewDASer` calls to remove the fraud service parameter.

- [ ] **Step 1: Modify `das/daser_test.go`**

Remove the `fraudtest` import:

```go
// Remove this line:
"github.com/celestiaorg/go-fraud/fraudtest"
```

Update `createDASerSubcomponents` (lines 222-238) to no longer return a fraud dummy service:

```go
func createDASerSubcomponents(
	t *testing.T,
	numGetter,
	numSub int,
) (
	libhead.Store[*header.ExtendedHeader],
	libhead.Subscriber[*header.ExtendedHeader],
) {
	mockGet, sub := createMockGetterAndSub(t, numGetter, numSub)
	return mockGet, sub
}
```

Update all `NewDASer` calls throughout the file to remove the fraud service argument. The parameter that was between `ds` (datastore) and `newBroadcastMock(1)` (shrexsub broadcast fn) should be removed:

- Line 39: `NewDASer(avail, sub, mockGet, ds, mockService, newBroadcastMock(1))` becomes `NewDASer(avail, sub, mockGet, ds, newBroadcastMock(1))`
- Line 70: same change
- Line 91: same change
- Line 208: `NewDASer(avail, sub, getter, ds, fserv, newBroadcastMock(1), ...)` becomes `NewDASer(avail, sub, getter, ds, newBroadcastMock(1), ...)`

For `createDASerSubcomponents` callers, update to receive two return values instead of three:

- Line 34: `mockGet, sub, mockService := createDASerSubcomponents(t, 15, 15)` becomes `mockGet, sub := createDASerSubcomponents(t, 15, 15)`
- Line 65: same change

Also remove the entire commented-out `TestDASer_stopsAfter_BEFP` function (lines 108-177) and its TODO comment.

Remove the `fserv` variable in `TestDASerSampleTimeout` (line 205): `fserv := &fraudtest.DummyService[*header.ExtendedHeader]{}` — delete this line, and update the `NewDASer` call on line 208.

- [ ] **Step 2: Run the DASer tests**

```bash
go test ./das/ -count=1 -v -timeout 30s
```

Expected: PASS for all DASer tests

- [ ] **Step 3: Commit**

```bash
git add das/daser_test.go
git commit -m "refactor: update DASer tests to remove fraud service dependency

Remove fraudtest.DummyService usage, update NewDASer calls to match
new signature without fraud broadcaster, and delete commented-out
BEFP test."
```

---

### Task 7: Remove Fraud from DAS Module (`nodebuilder/das/`)

**Files:**
- Modify: `nodebuilder/das/constructors.go`
- Modify: `nodebuilder/das/module.go`

Remove `ServiceBreaker` wrapping from the DASer constructor and simplify the module lifecycle hooks.

- [ ] **Step 1: Modify `nodebuilder/das/constructors.go`**

Remove imports for `go-fraud`, `nodebuilder/fraud`, and `byzantine`. Simplify `newDASer` to return only `*das.DASer`:

```go
package das

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
)

var _ Module = (*daserStub)(nil)

var errStub = fmt.Errorf("module/das: stubbed: dasing is not available on bridge nodes")

// daserStub is a stub implementation of the DASer that is used on bridge nodes, so that we can
// provide a friendlier error when users try to access the daser over the API.
type daserStub struct{}

func (d daserStub) SamplingStats(context.Context) (das.SamplingStats, error) {
	return das.SamplingStats{}, errStub
}

func (d daserStub) WaitCatchUp(context.Context) error {
	return errStub
}

func newDaserStub() Module {
	return &daserStub{}
}

func newDASer(
	da share.Availability,
	hsub libhead.Subscriber[*header.ExtendedHeader],
	store libhead.Store[*header.ExtendedHeader],
	batching datastore.Batching,
	bFn shrexsub.BroadcastFn,
	options ...das.Option,
) (*das.DASer, error) {
	return das.NewDASer(da, hsub, store, batching, bFn, options...)
}
```

- [ ] **Step 2: Modify `nodebuilder/das/module.go`**

Remove the `modfraud` import. Replace `ServiceBreaker` lifecycle hooks with direct DASer start/stop:

```go
package das

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// If DASer is disabled, provide the stub implementation for any node type
	// Also provide the stub implementation for bridge nodes as they do not need DASer
	if !cfg.Enabled || tp == node.Bridge {
		return fx.Module(
			"das",
			fx.Provide(newDaserStub),
		)
	}

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfg.Validate()),
		fx.Provide(
			func(c Config) []das.Option {
				return []das.Option{
					das.WithSamplingRange(c.SamplingRange),
					das.WithConcurrencyLimit(c.ConcurrencyLimit),
					das.WithBackgroundStoreInterval(c.BackgroundStoreInterval),
					das.WithSampleTimeout(c.SampleTimeout),
				}
			},
		),
	)

	return fx.Module(
		"das",
		baseComponents,
		fx.Provide(fx.Annotate(
			newDASer,
			fx.OnStart(func(ctx context.Context, daser *das.DASer) error {
				return daser.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, daser *das.DASer) error {
				return daser.Stop(ctx)
			}),
		)),
		// Module is needed for the RPC handler
		fx.Provide(func(das *das.DASer) Module {
			return das
		}),
	)
}
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./nodebuilder/das/...
```

Expected: success

- [ ] **Step 4: Commit**

```bash
git add nodebuilder/das/constructors.go nodebuilder/das/module.go
git commit -m "refactor: remove ServiceBreaker wrapping from DASer module

Simplify the DASer constructor and module to use direct start/stop
lifecycle hooks instead of wrapping with ServiceBreaker."
```

---

### Task 8: Remove Fraud from Header Module (`nodebuilder/header/`)

**Files:**
- Modify: `nodebuilder/header/constructors.go`
- Modify: `nodebuilder/header/module.go`
- Modify: `nodebuilder/header/service.go`

Remove `newFraudedSyncer`, `ServiceBreaker` wrapping, and update the header service constructor to take a raw Syncer.

- [ ] **Step 1: Modify `nodebuilder/header/constructors.go`**

Remove the `libfraud` import (`github.com/celestiaorg/go-fraud`), the `modfraud` import (`nodebuilder/fraud`), and the `byzantine` import. Delete the `newFraudedSyncer` function (lines 109-118).

The imports should become:

```go
import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/store"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)
```

- [ ] **Step 2: Modify `nodebuilder/header/module.go`**

Remove the `modfraud` import. Replace the `newFraudedSyncer` provider with direct Syncer lifecycle hooks.

Replace lines 39-60 (the `newFraudedSyncer` fx.Provide block) with:

```go
		fx.Provide(fx.Annotate(
			newSyncer[H],
			fx.OnStart(func(
				ctx context.Context,
				syncer *sync.Syncer[H],
			) error {
				// TODO(@Wondertan): This fix flakes in e2e tests
				//  This is coming from the store asynchronity.
				//  Previously, we would request genesis during initialization
				//  but now we request it during Syncer start and given to the Store.
				//  However, the Store doesn't makes it immediately available causing flakes
				//  The proper fix will be in a follow up release after pruning.
				defer time.Sleep(time.Millisecond * 100)
				return syncer.Start(ctx)
			}),
			fx.OnStop(func(
				ctx context.Context,
				syncer *sync.Syncer[H],
			) error {
				return syncer.Stop(ctx)
			}),
		)),
```

Note: Keep the `time` import since it's still used for the sleep workaround.

- [ ] **Step 3: Modify `nodebuilder/header/service.go`**

Remove the `modfraud` import. Update `newHeaderService` to accept `*sync.Syncer[*header.ExtendedHeader]` directly instead of `*modfraud.ServiceBreaker[...]`:

```go
package header

import (
	"context"
	"errors"
	"fmt"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
)
```

Update `newHeaderService`:

```go
func newHeaderService(
	syncer *sync.Syncer[*header.ExtendedHeader],
	sub libhead.Subscriber[*header.ExtendedHeader],
	p2pServer *p2p.ExchangeServer[*header.ExtendedHeader],
	ex libhead.Exchange[*header.ExtendedHeader],
	store libhead.Store[*header.ExtendedHeader],
) Module {
	return &Service{
		syncer:    syncer,
		sub:       sub,
		p2pServer: p2pServer,
		ex:        ex,
		store:     store,
	}
}
```

- [ ] **Step 4: Verify compilation**

```bash
go build ./nodebuilder/header/...
```

Expected: success

- [ ] **Step 5: Commit**

```bash
git add nodebuilder/header/constructors.go nodebuilder/header/module.go nodebuilder/header/service.go
git commit -m "refactor: remove ServiceBreaker wrapping from header syncer

Remove newFraudedSyncer and update the header module to use direct
Syncer lifecycle hooks. The header service now takes a raw Syncer
instead of a ServiceBreaker-wrapped one."
```

---

### Task 9: Remove Fraud from State Module (`nodebuilder/state/`)

**Files:**
- Modify: `nodebuilder/state/core.go`
- Modify: `nodebuilder/state/module.go`

Remove `ServiceBreaker` wrapping from the CoreAccessor constructor and simplify module lifecycle.

- [ ] **Step 1: Modify `nodebuilder/state/core.go`**

Remove `libfraud` import, `modfraud` import, and `byzantine` import. Simplify `coreAccessor` to no longer return a ServiceBreaker:

```go
package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"google.golang.org/grpc"

	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/state"
)

// coreAccessor constructs a new instance of state.Module over
// a celestia-core connection.
func coreAccessor(
	cfg Config,
	keyring keyring.Keyring,
	keyname AccountName,
	sync *sync.Syncer[*header.ExtendedHeader],
	network p2p.Network,
	client *grpc.ClientConn,
	additionalConns core.AdditionalCoreConns,
) (
	*state.CoreAccessor,
	Module,
	error,
) {
	var opts []state.Option
	if len(additionalConns) > 0 {
		opts = append(opts, state.WithAdditionalCoreEndpoints(additionalConns))
	}
	if cfg.EstimatorAddress != "" {
		opts = append(opts, state.WithEstimatorService(cfg.EstimatorAddress))

		if cfg.EnableEstimatorTLS {
			opts = append(opts, state.WithEstimatorServiceTLS())
		}
	}

	ca, err := state.NewCoreAccessor(keyring, string(keyname), sync, client, network.String(), opts...)
	return ca, ca, err
}
```

Note: The `fraudServ libfraud.Service[*header.ExtendedHeader]` parameter is removed, and the `*modfraud.ServiceBreaker[...]` return value is removed.

- [ ] **Step 2: Modify `nodebuilder/state/module.go`**

Remove the `modfraud` import. Replace `ServiceBreaker` lifecycle hooks with direct `CoreAccessor` start/stop:

Replace lines 32-44 (the `fxutil.ProvideIf(coreCfg.IsEndpointConfigured(), ...)` block) with:

```go
		fxutil.ProvideIf(coreCfg.IsEndpointConfigured(), fx.Annotate(
			coreAccessor,
			fx.OnStart(func(ctx context.Context, ca *state.CoreAccessor) error {
				return ca.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, ca *state.CoreAccessor) error {
				return ca.Stop(ctx)
			}),
		)),
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./nodebuilder/state/...
```

Expected: success

- [ ] **Step 4: Commit**

```bash
git add nodebuilder/state/core.go nodebuilder/state/module.go
git commit -m "refactor: remove ServiceBreaker wrapping from state CoreAccessor

Simplify the CoreAccessor constructor and state module to use direct
start/stop lifecycle hooks instead of wrapping with ServiceBreaker."
```

---

### Task 10: Remove Fraud from P2P PubSub (`nodebuilder/p2p/pubsub.go`)

**Files:**
- Modify: `nodebuilder/p2p/pubsub.go`

Remove fraud-related pubsub topic scoring and the `Unmarshaler` field from `pubSubParams`.

- [ ] **Step 1: Modify `nodebuilder/p2p/pubsub.go`**

Remove the `go-fraud` and `go-fraud/fraudserv` imports:

```go
// Remove these lines:
"github.com/celestiaorg/go-fraud"
"github.com/celestiaorg/go-fraud/fraudserv"
```

Also remove the `header` import (`github.com/celestiaorg/celestia-node/header`) since it's only used for the fraud Unmarshaler type parameter.

Remove the `Unmarshaler` field from `pubSubParams`:

```go
type pubSubParams struct {
	fx.In

	Ctx           context.Context
	Host          hst.Host
	Bootstrappers Bootstrappers
	Network       Network
}
```

Remove the fraud topic loop from `topicScoreParams`:

```go
func topicScoreParams(params pubSubParams) map[string]*pubsub.TopicScoreParams {
	return map[string]*pubsub.TopicScoreParams{
		headp2p.PubsubTopicID(params.Network.String()): &headp2p.GossibSubScore,
	}
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./nodebuilder/p2p/...
```

Expected: success

- [ ] **Step 3: Commit**

```bash
git add nodebuilder/p2p/pubsub.go
git commit -m "refactor: remove fraud pubsub topic scoring from P2P

Remove fraud-related gossipsub topic scoring and the fraud
ProofUnmarshaler dependency from pubsub params."
```

---

### Task 11: Remove Fraud from Node Wiring (`nodebuilder/`)

**Files:**
- Modify: `nodebuilder/module.go`
- Modify: `nodebuilder/node.go`
- Modify: `nodebuilder/settings.go`
- Modify: `nodebuilder/default_services.go`
- Modify: `nodebuilder/rpc/constructors.go`

Remove the fraud module from node assembly, the `FraudServ` field, fraud metrics, fraud API mapping, and fraud RPC registration.

- [ ] **Step 1: Modify `nodebuilder/module.go`**

Remove the fraud import and the `fraud.ConstructModule(tp),` line (line 52):

Remove from imports:

```go
// Remove this line:
"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
```

Remove from `baseComponents`:

```go
// Remove this line:
fraud.ConstructModule(tp),
```

- [ ] **Step 2: Modify `nodebuilder/node.go`**

Remove the fraud import and the `FraudServ` field (line 74):

Remove from imports:

```go
// Remove this line:
"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
```

Remove from the Node struct:

```go
// Remove this line:
FraudServ     fraud.Module  // not optional
```

- [ ] **Step 3: Modify `nodebuilder/settings.go`**

Remove the `go-fraud` import and the `fraud.WithMetrics` invocation (line 95):

Remove from imports:

```go
// Remove this line:
"github.com/celestiaorg/go-fraud"
```

Also remove the `header` import if it becomes unused (it's used on line 95 as a type param for `fraud.WithMetrics[*header.ExtendedHeader]`). Check if `header` is used elsewhere in the file — it's also imported at line 26 and used on line 95 only. Remove it.

Remove from `WithMetrics`:

```go
// Remove this line:
fx.Invoke(fraud.WithMetrics[*header.ExtendedHeader]),
```

- [ ] **Step 4: Modify `nodebuilder/default_services.go`**

Remove the fraud import and the fraud API mapping:

Remove from imports:

```go
// Remove this line:
"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
```

Remove from `PackageToAPI`:

```go
// Remove this line:
"fraud":      &fraud.API{},
```

- [ ] **Step 5: Modify `nodebuilder/rpc/constructors.go`**

Remove the fraud import, the `fraudMod` parameter, and the fraud service registration:

Remove from imports:

```go
// Remove this line:
"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
```

Update `registerEndpoints` to remove the fraud parameter and registration:

```go
func registerEndpoints(
	stateMod state.Module,
	shareMod share.Module,
	headerMod header.Module,
	daserMod das.Module,
	p2pMod p2p.Module,
	nodeMod node.Module,
	blobMod blob.Module,
	daMod da.Module, //nolint: staticcheck
	blobstreamMod blobstream.Module,
	serv *rpc.Server,
) {
	serv.RegisterService("das", daserMod, &das.API{})
	serv.RegisterService("header", headerMod, &header.API{})
	serv.RegisterService("state", stateMod, &state.API{})
	serv.RegisterService("share", shareMod, &share.API{})
	serv.RegisterService("p2p", p2pMod, &p2p.API{})
	serv.RegisterService("node", nodeMod, &node.API{})
	serv.RegisterService("blob", blobMod, &blob.API{})
	serv.RegisterService("da", daMod, &da.API{})
	serv.RegisterService("blobstream", blobstreamMod, &blobstream.API{})
}
```

- [ ] **Step 6: Verify compilation**

```bash
go build ./nodebuilder/...
```

Expected: success

- [ ] **Step 7: Commit**

```bash
git add nodebuilder/module.go nodebuilder/node.go nodebuilder/settings.go nodebuilder/default_services.go nodebuilder/rpc/constructors.go
git commit -m "refactor: remove fraud module from node wiring

Remove fraud module registration, FraudServ field, fraud metrics,
fraud API mapping, and fraud RPC endpoint registration."
```

---

### Task 12: Remove Fraud from Docgen Examples (`api/docgen/examples.go`)

**Files:**
- Modify: `api/docgen/examples.go`

Remove fraud-related example values and imports.

- [ ] **Step 1: Modify `api/docgen/examples.go`**

Remove these imports:

```go
// Remove this line:
"github.com/celestiaorg/go-fraud"
```

And remove the `byzantine` import:

```go
// Remove this line:
"github.com/celestiaorg/celestia-node/share/eds/byzantine"
```

Remove the `BadEncoding` example (line 81):

```go
// Remove this line:
add(byzantine.BadEncoding)
```

Remove the fraud Proof example (lines 83-92):

```go
// Remove these lines:
// TODO: this case requires more debugging, simple to leave it as it was.
exampleValues[reflect.TypeOf((*fraud.Proof[*header.ExtendedHeader])(nil)).Elem()] = byzantine.CreateBadEncodingProof(
	[]byte("bad encoding proof"),
	42,
	&byzantine.ErrByzantine{
		Index:  0,
		Shares: []*byzantine.ShareWithProof{},
		Axis:   rsmt2d.Axis(0),
	},
)
```

After removing the fraud proof example, check if `rsmt2d` is still needed by other example values. It is: `add(rsmt2d.Row)` on line 97 still uses it. Keep `rsmt2d`.

After removing the `header` import check: `header` is still used for ExtendedHeader unmarshal and by other references. Keep `header`.

After removing the `byzantine` import check: `byzantine` was only used for `BadEncoding` and `CreateBadEncodingProof` and `ErrByzantine`/`ShareWithProof`. After removal, the import is unused.

- [ ] **Step 2: Verify compilation**

```bash
go build ./api/docgen/...
```

Expected: success

- [ ] **Step 3: Commit**

```bash
git add api/docgen/examples.go
git commit -m "refactor: remove fraud proof examples from docgen

Remove BadEncoding proof type and fraud.Proof example values from
the API documentation generator."
```

---

### Task 13: Update Retriever Tests (`share/eds/retriever_test.go`)

**Files:**
- Modify: `share/eds/retriever_test.go`

Remove BEFP creation and validation from retriever tests. The tests should still verify that `ErrByzantine` is correctly detected, but not create or validate a `BadEncodingProof`.

- [ ] **Step 1: Modify `share/eds/retriever_test.go`**

In `TestRetriever_ByzantineError` (around line 100), simplify the test body to verify `ErrByzantine` without creating a BEFP:

Replace the test case body (lines 107-114):

```go
// Old:
var errByz *byzantine.ErrByzantine
faultHeader, err := generateByzantineError(ctx, t, size, bServ)
require.True(t, errors.As(err, &errByz))

p := byzantine.CreateBadEncodingProof([]byte("hash"), faultHeader.Height(), errByz)
err = p.Validate(faultHeader)
require.NoError(t, err)
```

With:

```go
// New:
var errByz *byzantine.ErrByzantine
_, err := generateByzantineError(ctx, t, size, bServ)
require.ErrorAs(t, err, &errByz)
```

In `BenchmarkBEFPValidation` (around line 141), either remove the benchmark entirely or update it to only benchmark ErrByzantine detection without BEFP creation/validation. Since it benchmarks BEFP validation specifically, remove it entirely.

Delete the entire `BenchmarkBEFPValidation` function (lines 141-169).

Remove the `header` import if it becomes unused (check: `headertest.ExtendedHeaderFromEDS` is used in `generateByzantineError` which uses `*header.ExtendedHeader`; the `header` package is imported as `"github.com/celestiaorg/celestia-node/header"` — check if it's used after changes).

Actually, looking more carefully at the imports: `header` is not directly imported in this file. The `headertest` package is. And `header` the package is used by `generateByzantineError` which returns `*header.ExtendedHeader`. Let me re-check the imports:

The file imports `"github.com/celestiaorg/celestia-node/header"` (line 17) and uses it in `generateByzantineError` return type. After our changes, `faultHeader` is no longer used (we use `_`), but `generateByzantineError` still returns `*header.ExtendedHeader` so the function signature still uses it. The import is still needed by `generateByzantineError`.

- [ ] **Step 2: Run the retriever tests**

```bash
go test ./share/eds/ -run TestRetriever -count=1 -v -timeout 30s
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add share/eds/retriever_test.go
git commit -m "refactor: simplify retriever tests to remove BEFP validation

Update TestRetriever_ByzantineError to only verify ErrByzantine is
detected without creating/validating a BadEncodingProof. Remove the
BEFP validation benchmark."
```

---

### Task 14: Remove `go-fraud` from `go.mod` and Tidy

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Remove the go-fraud dependency from go.mod**

```bash
go mod edit -droprequire github.com/celestiaorg/go-fraud
```

- [ ] **Step 2: Run go mod tidy**

```bash
go mod tidy
```

Expected: success, removes go-fraud and any now-unused transitive dependencies from go.sum

- [ ] **Step 3: Verify the full project builds**

```bash
go build ./...
```

Expected: success

- [ ] **Step 4: Run go vet**

```bash
go vet ./...
```

Expected: success (no issues)

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum
git commit -m "refactor: remove go-fraud dependency from go.mod

Remove github.com/celestiaorg/go-fraud and tidy module dependencies."
```

---

### Task 15: Run Full Test Suite and Verify

**Files:** None (verification only)

- [ ] **Step 1: Run the full test suite**

```bash
go test ./... -count=1 -timeout 10m
```

Expected: All tests PASS. No compilation errors. No references to go-fraud remaining.

- [ ] **Step 2: Verify no remaining references to go-fraud**

```bash
grep -r "go-fraud" --include="*.go" . | grep -v vendor | grep -v ".git"
```

Expected: No output (no remaining references)

- [ ] **Step 3: Verify no remaining references to nodebuilder/fraud**

```bash
grep -r "nodebuilder/fraud" --include="*.go" . | grep -v vendor | grep -v ".git"
```

Expected: No output

- [ ] **Step 4: Verify no remaining references to BadEncodingProof type**

```bash
grep -r "BadEncodingProof\|BadEncoding\b" --include="*.go" . | grep -v vendor | grep -v ".git" | grep -v "_test.go"
```

Expected: No output (or only comments/strings, not type references)

- [ ] **Step 5: Fix any remaining issues**

If any tests fail or references remain, fix them before proceeding.

- [ ] **Step 6: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "refactor: fix remaining fraud proof references"
```
