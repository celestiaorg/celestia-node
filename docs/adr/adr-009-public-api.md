# ADR 009: Public API

## Authors
@Wondertan @renaynay

## Changelog

- 2022-03-08: initial version
- 2022-08-03: additional details

## Context

Celestia Node has been built for almost half a year with a bottom-up approach to development. The core lower 
level components are built first and public API around them is getting organically shaped. Now that the project is 
maturing and its architecture is better defined, it's a good time to formally define a set of modules provided by the 
node and their respective APIs.

## Alternative Approaches

### Node Type centric Design
Another discussed approach to defining an API could be an onion style, where each new layer is a feature set of a broader
node type. However, it turns out that this approach does not seem to match the reality we have, where each node type
implements all the possible APIs but with the variable implementations matching resource constraints of a type.

## Design

### Goals
* Ergonomic. Simple, idiomatic and self-explanatory.
* Module centric(modular). The API is not monolithic and is segregated into different categorized and independent modules.
* Unified. All the node types implement the same set of APIs. The difference is defined by different implementations of
some modules to meet resource requirements of a type. Example: FullAvailability and LightAvailability
* Embeddable. Simply constructable Node with library style API. Not an SDK/Framework which dictates users the way to build
an app, but users are those who decide how they want to build the app using the API.
* Language agnostic. It should be simple to implement similar module interfaces/traits in other languages over RPC clients

### Implementation

The tracking issue can be found [here](https://github.com/celestiaorg/celestia-node/issues/944).
It provides a more detailed step-by-step process for how the below described design will be implemented.

### Details
#### Services Deprecation
The initial step is to deprecate services in favor of modules. Ex. `HeaderService` -> `HeaderModule`.

* We're organically moving towards the direction of modularized libraries. That is, our `share`, `header` and `state`
packages are getting shaped as independent modules which now lack their own API definition. 
* Consistency. Semantically, modules are closer to what high level Celestia project goals and slogans.
* Disassociate centralization. Services have always been associated with centralized infrastructure with older 
monolithic services and newer distributed microservice architectures. 

#### Modules Definition

##### Header
```go
type HeaderModule interface {
	// Head returns the node's local head (tip of the chain of the header store).
	Head(ctx context.Context) (*header.ExtendedHeader, error)
	// Get returns the header of the given hash from the node's header store.
	Get(ctx context.Context, hash tmbytes.HexBytes) (*header.ExtendedHeader, error)
	// GetByHeight returns the header of the given height from the node's header store.
	// If the header of the given height is not yet available, the request will hang
	// until it becomes available in the node's header store.
	GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error)
	// GetRangeByHeight returns the given range [from:to) of ExtendedHeaders.
	GetRangeByHeight(ctx context.Context, from, to uint64) ([]*ExtendedHeader, error)
	// Subscribe creates long-living Subscription for validated ExtendedHeaders.
	// Multiple Subscriptions can be created.
	Subscribe() (Subscription, error)    
	// SyncState returns the current state of the header Syncer. 
	SyncState() sync.State
	// SyncWait blocks until the header Syncer is synced to network head. 
	SyncWait(ctx context.Context) error
	// SyncHead provides the Syncer's view of the current network head.
	SyncHead(ctx context.Context) (*header.ExtendedHeader, error)

    EnableMetrics()
}
```

##### Shares
```go
type SharesModule interface {
	// GetShare returns the Share from the given data Root at the given row/col coordinates.
	GetShare(ctx context.Context, dah *Root, row, col int) (Share, error)
	// GetSharesByNamespace returns all shares of the given nID from the given data Root.
	GetSharesByNamespace(ctx context.Context, root *Root, nID namespace.ID) ([]Share, error)
	// SharesAvailable subjectively validates if Shares committed to the given data Root are 
	// available on the network. 
	SharesAvailable(ctx context.Context, root *Root) error
	// ProbabilityOfAvailability calculates the probability of the data square
	// being available based on the number of samples collected.
	ProbabilityOfAvailability() float64
	
    EnableMetrics()
}
```

##### P2P
```go
type P2PModule interface {
    Info
    Peers
    PeerInfo

    Connect
    Disconnect
    Connectedness
    NATStatus

    BlockAdd
    BlockRm
    BlockList

    MutualAdd
    MutualRm
    MutualList

    BandwidthStats
    BandwidthPeer
    BandwidthProtocol

    ResourceState
    ResourceLimit
    ResourceSetLimit

    PubSubPeers
    PubSubScore

    BitSwapStats
    BitSwapLedger

    DHTFindPeer
}
```

##### Wallet
```go
type WalletModule interface {
    New
    Del
    Has
    List
    Sign
    Export
    Import
}
```

### NodeModule
```go
type NodeModule interface {
    Type
    Version

    LogList
    LogLevelSet

	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
```

##### DAS
// TODO: Should we rebrand DASer to ShareSync?
```go
type DASModule interface {
	// State reports the current state of the DASer.
    State() das.SamplingState
	EnableMetrics()
}
```

##### State
```go
type StateModule interface {
    // Balance retrieves the Celestia coin balance for the node's account/signer
    // and verifies it against the corresponding block's AppHash.
    Balance(ctx context.Context) (*state.Balance, error)
    // BalanceForAddress retrieves the Celestia coin balance for the given address and verifies
    // the returned balance against the corresponding block's AppHash.
    BalanceForAddress(ctx context.Context, addr state.Address) (*state.Balance, error)
    // SubmitTx submits the given transaction/message to the Celestia network and blocks until 
	// the tx is included in a block.
    SubmitTx(ctx context.Context, tx state.Tx) (*state.TxResponse, error)
    // SubmitPayForData builds, signs and submits a PayForData transaction.
    SubmitPayForData(ctx context.Context, nID namespace.ID, data []byte, gasLimit uint64) (*state.TxResponse, error)
    // Transfer sends the given amount of coins from default wallet of the node to the given account address.
    Transfer(ctx context.Context, to types.Address, amount types.Int, gasLimit uint64) (*state.TxResponse, error)
}
```

Ideally all the state modules below should be implemented on top of only StateModule, but there is no way we can have
an abstract state requesting method, yet.

###### Staking
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

##### Fraud
```go
type FraudModule interface {
    Subscribe(type)
	Get(type)
    List(type)
	EnableMetrics()
}
```

## Status

Proposed
