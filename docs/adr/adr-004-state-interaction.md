# ADR #004: Celestia-Node State Interaction for March 2022 Testnet

<hr style="border:3px solid gray"> </hr>

## Authors

@renaynay

## Changelog

* 2022-01-14: initial draft

<hr style="border:2px solid gray"> </hr>

## Context

Currently, celestia-node lacks a way for users to submit a transaction to celestia-core as well as access its own state
and public state of others.

## Decision

Both celestia **light** and **full** nodes will run `StateService`. 
`StateService` will be responsible for managing the RPC endpoints provided by `StateAccessor`.

`StateAccessor` will be an interface for interacting with celestia-core in order to retrieve account-related information
as well as submit transactions.

### `StateService`

`StateService` will expose several higher-level RPC endpoints for users to access the methods provided by the 
`StateAccessor`.

```go
type StateService struct {
    accessor StateAccessor
}
``` 

### `StateAccessor`

`StateAccessor` is defined by the two methods listed below and may be expanded in the future to include more methods 
related to accounts and transactions in later iterations.

```go
type StateAccessor interface {
    func AccountBalance(context.Context, Account) (Balance, error)
    func SubmitTx(context.Context, Tx) error
}
```

`StateAccessor` will have 3 separate implementations under the hood, the first of which will be outlined in this ADR: 
1. **CORE**: RPC connection with a celestia-core endpoint handed to the node upon initialisation
(*required for this iteration*) 
2. **P2P**: discovery of a **bridge** node or other node that is able to provide state (*nice-to-have for this iteration*)
3. **LOCAL**: eventually, **full** nodes will be able to provide state to the network, and can therefore execute and 
respond to the queries without relaying queries to celestia-core (*to be scoped out and implemented in later iterations*)

### 1. Core Implementation of `StateAccessor`: `CoreAccess`

`CoreAccess` will be the direct RPC connection implementation of `StateAccessor`. It will use the [lens implementation of ChainClient](https://github.com/strangelove-ventures/lens/blob/main/client/chain_client.go#L23)
under the hood. `lens.ChainClient` provides a nice wrapper around the Cosmos-SDK APIs. 

Upon node initialisation, the node will receive the endpoint of a trusted celestia-core node. The constructor for 
`CoreAccess` will construct a new instance of `CoreAccess` with the `lens.ChainClient` pointing at the given 
celestia-core endpoint.

```go
type CoreAccess struct {
    cc *lens.ChainClient
}

func NewCoreAccess(cc *lens.ChainClient) (*CoreAccess, error) {
	return &CoreAccess{
        cc: cc,		
    }, nil   
}

// where Account wraps account information and Balance 
func (ca *CoreAccess) AccountBalance(ctx context.Context, acct Account) (Balance, error) {
	coins, err := da.cc.QueryBalanceWithAddress(acct.Address)
	if err != nil {
		return err
    }   
    // returning first index as we only care for account's balance on celestia chain	
	return coins[0], nil 
}

// where Tx implements sdk.Msg
func (ca *CoreAccess) SubmitTx(ctx context.Context, tx Tx) (TxResponse, error) {
	return da.cc.SendMsg(ctx, tx)
}
```

### 2. P2P Implementation of `StateAccessor`: `P2PAccess`

While it is not necessary to detail how `P2PAccess` will be implemented in this ADR, it will still conform to the 
`StateAccessor` interface, but instead of being provided a core endpoint to connect to via RPC and using `lens.ChainClient`
for state-related queries, `P2PAccess` will perform service discovery of state-providing nodes in the network and perform
the state queries via libp2p streams.

```go
type P2PAccess struct {
    // for discovery and communication
    // with state-providing nodes
    host host.Host
    // for managing keys
    keybase        keyring.Keyring
    keyringOptions []keyring.Option
}
```

### `StateProvider`

A **bridge** node will run a `StateProvider` (server-side of `P2PAccessor`). The `StateProvider` will be responsible for
relaying the state-related queries through to its trusted celestia-core node.

A new `core.StateFetcher` will be implemented over the `core.Client` in the `core` package. The `core.StateFetcher` will
contain methods that wrap communication with the `core.Client`. 

`StateProvider` will listen for state-related queries from **light** nodes and will query the `core.StateFetcher` with 
the received payloads. 
