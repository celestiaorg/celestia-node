# ADR 009: Public API

## Authors

@Wondertan @renaynay

## Changelog

- 2022-03-08: initial version
- 2022-08-03: additional details

## Context

Celestia Node has been built for almost half a year with a bottom-up approach to
development. The core lower level components are built first and public API
around them is getting organically shaped. Now that the project is maturing and
its architecture is better defined, it's a good time to formally define a set of
modules provided by the node and their respective APIs.

## Alternative Approaches

### Node Type centric Design

Another discussed approach to defining an API could be an onion style, where
each new layer is a feature set of a broader node type. However, it turns out
that this approach does not seem to match the reality we have, where each node
type implements all the possible APIs but with the variable implementations
matching resource constraints of a type.

## Design

### Goals

- Ergonomic. Simple, idiomatic and self-explanatory.
- Module centric(modular). The API is not monolithic and is segregated into
different categorized and independent modules.
- Unified. All the node types implement the same set of APIs. The difference is
defined by different implementations of some modules to meet resource
requirements of a type. Example: FullAvailability and LightAvailability.
- Embeddable. Simply constructable Node with library style API. Not an
SDK/Framework which dictates users the way to build an app, but users are those
who decide how they want to build the app using the API.
- Language agnostic. It should be simple to implement similar module
interfaces/traits in other languages over RPC clients.

### Implementation

The tracking issue can be found
[here](https://github.com/celestiaorg/celestia-node/issues/944). It provides a
more detailed step-by-step process for how the below described design will be
implemented.

### High-level description

All node types in `celestia-node` will be referred to as `data availability
nodes (DA nodes)` whose sole purpose is to interact with the `data availability
network layer (DA layer)` such that the node contains all functionality
necessary to post and retrieve messages from the DA layer.

This means that DA nodes will be able to query for / modify celestia state such
that the DA nodes are able to pay for posting their messages on the network.
The state-related API will be documented below in detail.

Furthermore, interaction between the celestia consensus network and the
celestia data availability network will be the responsibility of the **bridge**
node type. However, that interaction will not be exposed on a public level
(meaning a **bridge** node will not expose the same API as the
celestia-core node to which it is connected). A **bridge** node, for all intents
and purposes, will provide the same API as that of a **full** node (with a
stubbed-out DAS module as bridge nodes do not perform sampling).

### Details

#### Services Deprecation

The initial step is to deprecate services in favor of modules. Ex.
`HeaderService` -> `HeaderModule`.

- We're organically moving towards the direction of modularized libraries. That
is, our `share`, `header` and `state` packages are getting shaped as independent
modules which now lack their own API definition.
- Consistency. Semantically, modules are closer to what Celestia's overarching
project goals and slogans.
- Disassociate centralization. Services have always been associated with
centralized infrastructure with older monolithic services and newer distributed
microservice architectures.

#### Modules Definition

##### Header

```go
type HeaderModule interface {
// LocalHead returns the node's local head (tip of the chain of the header store).
LocalHead(ctx context.Context) (*header.ExtendedHeader, error)
// GetByHash returns the header of the given hash from the node's header store.
GetByHash(ctx context.Context, hash tmbytes.HexBytes) (*header.ExtendedHeader, error)
// GetByHeight returns the header of the given height if it is available.
GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error)
// WaitForHeight blocks until the header at the given height has been processed
// by the node's header store or until context deadline is exceeded.
WaitForHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error)
// GetRangeByHeight returns the given range (from:to) of ExtendedHeaders
// from the node's header store and verifies that the returned headers are 
// adjacent to each other.
GetRangeByHeight(ctx context.Context, from, to uint64) ([]*ExtendedHeader, error)
// Subscribe creates long-living Subscription for newly validated 
// ExtendedHeaders. Multiple Subscriptions can be created.
Subscribe(context.Context) (<-chan *header.ExtendedHeader, error)
// SyncState returns the current state of the header Syncer. 
SyncState(context.Context) (sync.State, error)
// SyncWait blocks until the header Syncer is synced to network head. 
SyncWait(ctx context.Context) error
// NetworkHead provides the Syncer's view of the current network head.
NetworkHead(ctx context.Context) (*header.ExtendedHeader, error)
}

```

##### Shares

```go
  type SharesModule interface {
    // GetShare returns the Share from the given data Root at the given row/col
    // coordinates.
    GetShare(ctx context.Context, root *Root, row, col int) (Share, error)
    // GetEDS gets the full EDS identified by the given root.
    GetEDS(ctx context.Context, root *share.Root) (*rsmt2d.ExtendedDataSquare, error)
    // GetSharesByNamespace gets all shares from an EDS within the given namespace.
    // Shares are returned in a row-by-row order if the namespace spans multiple rows.
    GetSharesByNamespace(
      ctx context.Context,
      root *Root, 
      nID namespace.ID, 
    ) (share.NamespacedShares, error)
    // SharesAvailable subjectively validates if Shares committed to the given data
    // Root are available on the network. 
    SharesAvailable(ctx context.Context, root *Root) error
    // ProbabilityOfAvailability calculates the probability of the data square
    // being available based on the number of samples collected.
    ProbabilityOfAvailability() float64
  }
```

##### P2P

```go
  type P2PModule interface {
    // Info returns address information about the host.
    Info(context.Context) (peer.AddrInfo, error)
    // Peers returns all peer IDs used across all inner stores.
    Peers(context.Context) ([]peer.ID, error)
    // PeerInfo returns a small slice of information Peerstore has on the
    // given peer.
    PeerInfo(context.Context, peer.ID) (peer.AddrInfo, error)
   
    // Connect ensures there is a connection between this host and the peer with
    // given peer.
    Connect(ctx context.Context, pi peer.AddrInfo) error
    // ClosePeer closes the connection to a given peer. 
    ClosePeer(ctx context.Context, id peer.ID) error
    // Connectedness returns a state signaling connection capabilities.
    Connectedness(ctx context.Context, id peer.ID) network.Connectedness
    // NATStatus returns the current NAT status.
    NATStatus(context.Context) network.Reachability
   
    // BlockPeer adds a peer to the set of blocked peers.
    BlockPeer(ctx context.Context, p peer.ID) error
    // UnblockPeer removes a peer from the set of blocked peers.
    UnblockPeer(ctx context.Context, p peer.ID) error
    // ListBlockedPeers returns a list of blocked peers.
    ListBlockedPeers(context.Context) []peer.ID

    // MutualAdd adds a peer to the list of peers who have a bidirectional
    // peering agreement that they are protected from being trimmed, dropped
    // or negatively scored.
    MutualAdd(ctx context.Context, id peer.ID, tag string)
    // MutualAdd removes a peer from the list of peers who have a bidirectional
    // peering agreement that they are protected from being trimmed, dropped
    // or negatively scored, returning a bool representing whether the given
    // peer is protected or not.
    MutualRm(ctx context.Context, id peer.ID, tag string) bool
    // IsMutual returns whether the given peer is a mutual peer.
    IsMutual(ctx context.Context, id peer.ID, tag string) bool
  
    // BandwidthStats returns a Stats struct with bandwidth metrics for all
    // data sent/received by the local peer, regardless of protocol or remote 
    // peer IDs.
    BandwidthStats(context.Context) Stats
    // BandwidthForPeer returns a Stats struct with bandwidth metrics associated
    // with the given peer.ID. The metrics returned include all traffic sent /
    // received for the peer, regardless of protocol.
    BandwidthForPeer(ctx context.Context, id peer.ID) Stats
    // BandwidthForProtocol returns a Stats struct with bandwidth metrics 
    // associated with the given protocol.ID.
    BandwidthForProtocol(ctx context.Context, proto protocol.ID) Stats
   
    // ResourceState returns the state of the resource manager.
    ResourceState(context.Context) rcmgr.ResourceManagerStat
   
    // PubSubPeers returns the peer IDs of the peers joined on
    // the given topic.
    PubSubPeers(ctx context.Context, topic string) ([]peer.ID, error)
  }
```

### NodeModule

```go

  type NodeModule interface {
    // Info returns administrative information about the node.
    Info(context.Context) (Info, error)
 
    // LogLevelSet sets the given component log level to the given level.
    LogLevelSet(ctx context.Context, name, level string) error

    // AuthVerify returns the permissions assigned to the given token.
    AuthVerify(ctx context.Context, token string) ([]auth.Permission, error)
    // AuthNew signs and returns a new token with the given permissions.
    AuthNew(ctx context.Context, perms []auth.Permission) ([]byte, error)
  }
  
```

#### DAS

```go
  type DASModule interface {
    // Stats returns current stats of the DASer.
    Stats() (das.SamplingStats, error)
  }
```

#### State

```go
  type StateModule interface {
    // AccountAddress retrieves the address of the node's account/signer
    AccountAddress(ctx context.Context) (state.Address, error)
    // Balance retrieves the Celestia coin balance for the node's account/signer
    // and verifies it against the corresponding block's AppHash.
    Balance(ctx context.Context) (*state.Balance, error)
    // BalanceForAddress retrieves the Celestia coin balance for the given 
    // address and verifies the returned balance against the corresponding
    // block's AppHash.
    BalanceForAddress(ctx context.Context, addr state.Address) (*state.Balance, error)
    // SubmitTx submits the given transaction/message to the Celestia network
    // and blocks until the tx is included in a block.
    SubmitTx(ctx context.Context, tx state.Tx) (*state.TxResponse, error)
    // SubmitPayForBlob builds, signs and submits a PayForBlob transaction.
    SubmitPayForBlob(
      ctx context.Context, 
      nID namespace.ID, 
      data []byte,
      fee types.Int,
      gasLimit uint64,
    ) (*state.TxResponse, error)
    // Transfer sends the given amount of coins from default wallet of the node 
    // to the given account address.
    Transfer(
      ctx context.Context, 
      to types.Address,
      amount types.Int,
      fee types.Int,
      gasLimit uint64,
    ) (*state.TxResponse, error)

    // StateModule also provides StakingModule
    StakingModule
  }
```

Ideally all the state modules below should be implemented on top of only
StateModule, but there is no way we can have an abstract state requesting method,
yet.

##### Staking

```go
  type StakingModule interface {
    // Delegate sends a user's liquid tokens to a validator for delegation.
    Delegate(
        ctx context.Context, 
        delAddr state.ValAddress, 
        amount state.Int,
        fee types.Int,
        gasLim uint64,
    ) (*state.TxResponse, error)
    // BeginRedelegate sends a user's delegated tokens to a new validator for redelegation.
    BeginRedelegate(
        ctx context.Context,
        srcValAddr,
        dstValAddr state.ValAddress,
        amount state.Int,
        fee types.Int,
        gasLim uint64, 
    ) (*state.TxResponse, error)
    // Undelegate undelegates a user's delegated tokens, unbonding them from the
    // current validator.
    Undelegate(
        ctx context.Context, 
        delAddr state.ValAddress,
        amount state.Int,
        fee types.Int,
        gasLim uint64,
    ) (*state.TxResponse, error)

    // CancelUnbondingDelegation cancels a user's pending undelegation from a 
    // validator.
    CancelUnbondingDelegation(
        ctx context.Context,
        valAddr state.ValAddress,
        amount types.Int,
        height types.Int,
        fee types.Int,
        gasLim uint64,
    ) (*state.TxResponse, error)

    // QueryDelegation retrieves the delegation information between a delegator
    // and a validator.
    QueryDelegation(
        ctx context.Context, 
        valAddr state.ValAddress, 
    ) (*types.QueryDelegationResponse, error)
    // QueryRedelegations retrieves the status of the redelegations between a
    // delegator and a validator.
    QueryRedelegations(
        ctx context.Context,
        srcValAddr,
        dstValAddr state.ValAddress,
    ) (*types.QueryRedelegationsResponse, error)
    // QueryUnbonding retrieves the unbonding status between a delegator and a validator.
    QueryUnbonding(
        ctx context.Context,
        valAddr state.ValAddress,
    ) (*types.QueryUnbondingDelegationResponse, error)

  }
```

#### Fraud

```go
  type FraudModule interface {
    // Subscribe subscribes to the given fraud proof type.
    Subscribe(proof fraud.ProofType) error
    // List lists all proof types to which the fraud module is currently
    // subscribed.
    List() []fraud.ProofType

    // Get returns any stored fraud proofs of the given type.
    Get(proof fraud.ProofType) ([]Proof, error)
  }
```

#### Metrics

```go
  type MetricsModule interface {
    // List shows all the registered meters.
    List(ctx) (string[], error)
    
    // Enable turns on the specific meter.
    Enable(string) error
    Disable(string) error
    
    // ExportTo  sets the endpoint the metrics should be exported to
    ExportTo(string)
  }
```

### Nice to have (post-mainnet)

#### State-related modules

Eventually, it would be nice to break up `StateModule` into `StateModule`,
`BankModule` and `StakingModule`.

##### State (general)

```go
type StateModule interface {
    // QueryABCI proxies a generic ABCI query to the core endpoint.
    QueryABCI(ctx context.Context, request abci.RequestQuery) 
      (*coretypes.ResultABCIQuery, error)
    // SubmitTx submits the given transaction/message to the Celestia network
    // and blocks until the tx is included in a block.
    SubmitTx(ctx context.Context, tx state.Tx) (*state.TxResponse, error)
}
```

##### Bank

```go
type BankModule interface {
  // Balance retrieves the Celestia coin balance for the node's account/signer
  // and verifies it against the corresponding block's AppHash.
  Balance(ctx context.Context) (*state.Balance, error)
  // BalanceForAddress retrieves the Celestia coin balance for the given 
  // address and verifies the returned balance against the corresponding
  // block's AppHash.
  BalanceForAddress(ctx context.Context, addr state.Address) (*state.Balance, error)
  // SubmitPayForBlob builds, signs and submits a PayForBlob transaction.
  SubmitPayForBlob(
    ctx context.Context,
    nID namespace.ID,
    data []byte,
    gasLimit uint64,
  ) (*state.TxResponse, error)
  // Transfer sends the given amount of coins from default wallet of the node 
  // to the given account address.
  Transfer(
    ctx context.Context,
    to types.Address,
    amount types.Int,
    gasLimit uint64,
  ) (*state.TxResponse, error)
}
```

##### Staking (same as pre-mainnet staking module)

```go
  type StakingModule interface {
    Delegate
    Redelegate
    Unbond
    CancelUnbond
    
    QueryDelegation
    QueryRedelegation
    QueryUnbondingDelegation
  }
```

##### Account

```go
  type AccountModule interface {
    Add
    Delete
    Show
    List
    Sign
    Export
    Import
  }
```

## Status

Proposed
