# ADR 007: Public API

## Authors
@Wondertan

## Changelog

- 2022-03-08: initial version

## Context

Celestia Node is being built for almost a half of a year with the bottom-up approach to development. The core lower 
level components are built first and public API around them is getting organically shaped. The project has raised some 
meat and the time to define the v0 API has come.

## Alternative Approaches

### Node Type centric Design
Another discussed approach to defining an API could be an onion style, where each new layer is a feature set of bigger TODO Needs a better word
node type. However, it turns out that this approach does not seem to match the reality we have, where each node type
implements all the possible APIs but with the variable implementations matching resource constraints of a type.

## Design

### Goals
* Ergonomic. Simple, idiomatic and self-explanatory.
* Module centric(modular). The API is not monolithic and segregated into different categorized and independent modules.
* Unified. All the node types implement the same set of APIs. The difference is defined by different implementations of
some modules to meet resource requirements of a type. Example: FullAvailability and LightAvailability
* Embeddable. Simply constructable Node with library style API. Not an SDK/Framework which dictates users the way to build
an app, but users are those who decide how they want to build the app using the API.
* Language agnostic. It should be simple to implement similar module interfaces/traits in other languages over RPC clients

### Details
#### Services Deprecation
The initial step is to deprecate and remove our services in code and stop relying on services as the notion. Instead, 
the Module should be used, e.g. HeaderService -> HeaderModule.

* We're organically moving towards the direction of modularized libraries. That is, our `share`, `header` and `state`
packages are getting shaped as independent sovereign modules which now lack their own API definition. 
* Consistency. Semantically, Modules are closer to what high level Celestia project goals and slogans.
* Disassociate centralization. Services have always been associated with centralized infrastructure with older 
monolithic services and newer distributed microservice architectures. 

#### Modules Definition
// TODO: Params and comment are yet to come
##### Header
```go
type HeaderModule interface {
    Head
    Get
    GetByHeight
    GetRangeByHeight
    Subscribe
    SyncState
    SyncWait
}
```

##### Shares
```go
type SharesModule interface {
	GetShare
	Get
	GetByNamespace
	IsAvailable
	Submit
}
```

##### P2P/Net(work)
```go
type P2PModule/NetworkModule interface {
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
    // TODO: Mnemonics stuff?
}
```

### NodeModule/MiscModule
```go
type NodeModule/MiscModule interface {
    Type
    Version

    LogList
    LogSet

    // TODO: Shutdown/Stop?

    // TODO: Auth

}
```

##### DAS
// TODO: Should we rebrand DASer to ShareSync?
```go
    State
    Wait
```

##### State
```go
type StateModule interface {
    Balance
    BalanceOf
    SubmitTx
    SubmitTxFrom

    // TODO: Feels like something is missing here
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

    // TODO Read methods

}
```

##### Fraud
```go
type FraudModule interface {
    Broadcast
    Subscribe(type)
    AddValidator(type)
    Register(type)
    Unregister(type)
    List(type)
}
```

## Status

Proposed

## Consequences
// TODO Are they required?
### Positive


### Negative
