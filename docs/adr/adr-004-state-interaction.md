# ADR #004: Celestia-Node State Interaction for March 2022 Testnet

<hr style="border:3px solid gray"> </hr>

## Authors

@renaynay

## Changelog

* 2022-01-14: initial draft
* 2022-02-11: update to interface and `CoreAccess` implementation details

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
   // Balance retrieves the Celestia coin balance
   // for the node's account/signer.
   Balance(ctx context.Context) (*Balance, error)
   // BalanceForAddress retrieves the Celestia coin balance
   // for the given address (given as `types.AccAddress` from cosmos-SDK).
   BalanceForAddress(ctx context.Context, addr types.AccAddress) (*Balance, error)
   // SubmitTx submits the given transaction/message to the
   // Celestia network and blocks until the tx is included in
   // a block.
   SubmitTx(ctx context.Context, tx Tx) (*TxResponse, error)
}
```

`StateAccessor` will have 3 separate implementations under the hood, the first of which will be outlined in this ADR:

1. **CORE**: RPC connection with a celestia-core endpoint handed to the node upon initialisation
   (*required for this iteration*)
2. **P2P**: discovery of a **bridge** node or other node that is able to provide state (*nice-to-have for this
   iteration*)
3. **LOCAL**: eventually, **full** nodes will be able to provide state to the network, and can therefore execute and
   respond to the queries without relaying queries to celestia-core (*to be scoped out and implemented in later
   iterations*)

### Verification of Balances

In order to check that the balances returned via the `AccountBalance` query are correct, it is necessary to also request
Merkle proofs from celestia-app and verify them against the latest head's `AppHash`.

In order for the `StateAccessor` to do this, it would need access to the `header.Getter`'s `Head()` method in order to get the latest known header of the node and check its `AppHash`.
Then, instead of performing a regular `gRPC` query against the celestia-app's bank module, it would perform an ABCI request query via RPC as such:

```go
  prefixedAccountKey := append(bank_types.CreateAccountBalancesPrefix(addr.Bytes()), []byte(app.BondDenom)...)
  abciReq := abci.RequestQuery{
      Path:   fmt.Sprintf("store/%s/key", bank_types.StoreKey),
      // we request the balance at the block *previous* to the current head as
      // the AppHash committed to the head is calculated by applying the previous
      // block's transaction list, not the current block, meaning the head's AppHash
      // is actually a result of the previous block's transactions.
      Height: head.Height - 1,
      Data:   prefixedAccountKey,
      // we set this value to `true` in order to get proofs in the response
      // that we can use to verify against the head's AppHash.
      Prove:  true,
  }
  result, err := ca.rpcCli.ABCIQueryWithOptions(ctx, abciReq.Path, abciReq.Data,  rpc_client.ABCIQueryOptions{})
  if err != nil {
      return nil, err
  }
```

The result of the above request will contain the balance of the queried address and proofs that can be used to verify
the returned balance against the current head's `AppHash`. The proofs are returned as the type `crypto.ProofOps` which
itself is not really functional until converted into a more useful wrapper, `MerkleProof`, provided by the `ibc-go`
[23-commitment/types pkg](https://github.com/cosmos/ibc-go/blob/main/modules/core/23-commitment/types/utils.go#L10).

Using `types.ConvertProofs()` returns a `types.MerkleProof` that wraps a chain of commitment proofs with which you can
verify the membership of the returned balance in the tree of the given root (the `AppHash` from the head), as such:

```go
 // convert proofs into a more digestible format
 merkleproof, err := proof_utils.ConvertProofs(result.Response.GetProofOps())
 if err != nil {
     return nil, err
 }
 root := proof_utils.NewMerkleRoot(head.AppHash)
 // VerifyMembership expects the path as:
 // []string{<store key of module>, <actual key corresponding to requested value>}
 path := proof_utils.NewMerklePath(bank_types.StoreKey, string(prefixedAccountKey))
 err = merkleproof.VerifyMembership(proof_utils.GetSDKSpecs(), root, path, result.Response.Value)
 if err != nil {
     return nil, err
 }
```

### Availability of `StateService` during sync

The `Syncer` in the `header`  package provides one public method, `Finished()`, that indicates whether the syncer has
finished syncing. Introducing the availability of `StateService` would require extending the public API for `Syncer`
with an additional method, `NetworkHead()`, in order to be able to fetch *current* state from the network. The `Syncer`
would then have to be passed to any implementation of `StateService` upon construction and relied on in order to access
the network head even if the syncer is still syncing, as the network head is still verified even during sync.

### 1. Core Implementation of `StateAccessor`: `CoreAccess`

`CoreAccess` will be the direct RPC connection implementation of `StateAccessor`. It will maintain a gRPC connection
with a celestia-core node under the hood.

Upon node initialisation, the node will receive the endpoint of a trusted celestia-core node. The constructor for
`CoreAccess` will create a new instance of `CoreAccess`, and the given celestia-core endpoint will only be dialed upon
start.

```go
type CoreAccess struct {
    signer *apptypes.KeyringSigner
    encCfg cosmoscmd.EncodingConfig

    coreEndpoint string
    coreConn     *grpc.ClientConn
}

func (ca *CoreAccessor) BalanceForAddress(ctx context.Context, addr string) (*Balance, error) {
    queryCli := banktypes.NewQueryClient(ca.coreConn)

    balReq := &banktypes.QueryBalanceRequest{
        Address: addr,
        Denom:   app.DisplayDenom,
    }

    balResp, err := queryCli.Balance(ctx, balReq)
    if err != nil {
        return nil, err
    }

    return balResp.Balance, nil
}

func (ca *CoreAccessor) SubmitTx(ctx context.Context, tx Tx) (*TxResponse, error) {
    txResp, err := apptypes.BroadcastTx(ctx, ca.coreConn, sdk_tx.BroadcastMode_BROADCAST_MODE_SYNC, tx)
    if err != nil {
        return nil, err
    }
    return txResp.TxResponse, nil
}

```

### 2. P2P Implementation of `StateAccessor`: `P2PAccess`

While it is not necessary to detail how `P2PAccess` will be implemented in this ADR, it will still conform to the
`StateAccessor` interface, but instead of being provided a core endpoint to connect to via RPC, `P2PAccess` will perform
service discovery of state-providing nodes in the network and perform the state queries via libp2p streams. More details
of the p2p implementation will be described in a separate dedicated ADR.

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

The `StateProvider` will be initialised with a celestia-core RPC connection. It will listen for inbound state-related
queries from its peers and relay the received payloads to celestia-core.
