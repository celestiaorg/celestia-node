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
    // AccountBalance returns the Celestia Balance for the given Account
    func AccountBalance(context.Context, Account) (Balance, error)
	// SubmitTx submits the given Tx, where Tx implements sdk.Msg
    func SubmitTx(context.Context, Tx) error
}
```

`StateAccessor` will have 3 separate implementations under the hood, the first of which will be outlined in this ADR: 
1. **CORE**: RPC connection with a celestia-core endpoint handed to the node upon initialisation
(*required for this iteration*) 
2. **P2P**: discovery of a **bridge** node or other node that is able to provide state (*nice-to-have for this iteration*)
3. **LOCAL**: eventually, **full** nodes will be able to provide state to the network, and can therefore execute and 
respond to the queries without relaying queries to celestia-core (*to be scoped out and implemented in later iterations*)

### Verification of Balances
In order to check that the balances returned via the `AccountBalance` query are correct, it is necessary to also request
Merkle proofs from the celestia-app and verify them against the latest head's `AppHash`. In order for the `StateAccessor`
to do this, it would need access to the `header.Store`'s `Head()` method in order to get the latest known header of the 
node and check its `AppHash`.

### Availability of `StateService` during sync
The `Syncer` in the `header`  package provides one public method, `Finished()`, that indicates whether the syncer has 
finished syncing. Introducing the availability of `StateService` would require extending the public API for `Syncer` 
with an additional method, `NetworkHead()`, in order to be able to fetch *current* state from the network. The `Syncer`
would then have to be passed to any implementation of `StateService` upon construction and relied on in order to access 
the network head even if the syncer is still syncing, as the network head is still verified even during sync.

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
the state queries via libp2p streams. More details of the p2p implementation will be described in a separate dedicated ADR.

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

#### `StateProvider`

A **bridge** node will run a `StateProvider` (server-side of `P2PAccessor`). The `StateProvider` will be responsible for
relaying the state-related queries through to its trusted celestia-core node.

The `StateProvider` will be initialised with a `lens.ChainClient` that will be used under the hood for communication 
with its already-configured celestia-core RPC connection. It will listen for inbound state-related queries from its 
peers and relay the received payloads to celestia-core.
