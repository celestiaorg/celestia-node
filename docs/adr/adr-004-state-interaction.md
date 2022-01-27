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

`StateAccessor` will have 3 separate implementations under the hood: 
1. **CORE**: RPC connection with a celestia-core endpoint handed to the node upon initialisation
(*required for this iteration*) 
2. **P2P**: discovery of a **bridge** node or other node that can provide state and submit messages via libp2p to be 
relayed to celestia-core (*nice-to-have for this iteration*)
3. **LOCAL**: eventually, **full** nodes will be able to provide state to the network, and can therefore execute and 
respond to the queries without relaying queries to celestia-core (*to be scoped out and implemented in later iterations*)

### `CoreAccess`

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
	coins, err := da.cc.QueryBalanceWithAddress(acct.address)
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

### `P2PAccess`

An idea for how `P2PAccess` could be implemented can be found below.

```go
type P2PAccess struct {
	host libhost.Host
    // P2PAccess will still use lens.ChainClient's methods
	// under the hood, but will use libp2p transport instead
	// of RPC communication with the Core endpoint.
   cc  *lens.ChainClient 
}

func NewP2PAccess(host libhost.Host, cc *lens.ChainClient) *P2PAccess {
    return &P2PAccess{
		host: host,
		cc: cc,
    }   	
}

func (pa *P2PAccess) AccountBalance(ctx context.Context, acct Account) (*Balance, error) {
    // passing in lens.ChainClient here to help construct the query
	query := acctBalanceQuery(pa.cc, acct)
	
    // open connection with peer who serves State
    stream, err := pa.ConnectToStateProvider()   
	if err != nil {
		return nil, err
    }

    // write query to stream
    err := stream.Write(query)
    if err != nil {
		return nil, err
    }

    // read resp from stream
    resp, err := stream.Read(buf)
    if err != nil {
        return nil, err		    
    }	

    // unmarshal balance from response
	var bal Balance
    err = bal.Unmarshal(buf)	
	if err != nil {
		return nil, err
    }
	
    return bal, nil
}

// where Tx implements sdk.Msg
func (pa *P2PAccess) SubmitTx(ctx context.Context, tx Tx) (*TxResponse, error) {
    // passing in lens.ChainClient here to help construct the query 
	query := submitTxQuery(pa.cc, tx)
	
    // open connection with peer who serves State
    stream, err := pa.ConnectToStateProvider()
    if err != nil {
        return nil, err
    }
    
    // write query to stream
    err := stream.Write(query)
    if err != nil {
		return nil, err
    }

    // read response from stream
    resp, err := stream.Read(buf)
    if err != nil {
        return nil, err
    }	
	
    // get txResponse from response
	var tx Tx
	err = tx.Unmarshal(buf)
	if err != nil {
		return nil, err
}
	
    return TxResponse, nil
}

```

### `StateProvider`

A **bridge** node will run a `StateProvider` (server-side of `P2PAccessor`). The `StateProvider` will be responsible for
relaying the state-related queries through to its trusted celestia-core node.

A new `core.StateFetcher` will be implemented over the `core.Client` in the `core` package. The `core.StateFetcher` will
contain methods that wrap communication with the `core.Client`. 

`StateProvider` will listen for state-related queries from **light** nodes and will query the `core.StateFetcher` with 
the received payloads. 
