# ADR 013: Bootstrap from previous peers

## Changelog

* 2023-05-04: initial draft

## Context

Celestia node relies on a set of peer to peer protocols that assume a decentralized network, where nodes are connected to each other and can exchange data.

Currently, when a node joins/comes up on the network, it relies, by default, on hardcoded bootstrappers for 3 things:

1. initializing its store with the first (genesis) block
2. requesting the network head to which it sets its initial sync target, and
3. bootstrapping itself with some other peers on the network which it can utilise for its operations.

This is centralisation bottleneck as it happens both during initial start-up and for any re-starts.

In this ADR, we wish to aleviate this problem by allowing nodes to bootstrap from previously seen peers. This is a simple solution that allows the network alleviate the start-up centralisation bottleneck such that nodes can join the network without the need for hardcoded bootstrappers up re-start

(_Original issue available in references_)

## Decision

Periodically store the addresses of previously seen peers in a key-value database, and allow the node to bootstrap from these peers on start-up.

## Detailed Design

### What systems will be affected?

* `peerTracker` and `libhead.Exchange` from `go-header`
* `newInitStore` in `nodebuilder/header.go`

### Database

We will use a badgerDB datastore to store the addresses of previously seen peers. The database will be stored in the `data` directory, and will be named `peers.db`. The database will have a single key-value pair, where the key is the peer ID, and the value is the peer multiaddress.

The peer store will implement the following interface:

```go
type PeerStore interface {
    // Put adds a peer to the store.
    Put(peer.AddrInfo) error
    // Get returns a peer from the store.
    Get() ([]peer.AddrInfo, error)
    // Delete removes a peer from the store.
    Delete(peer.ID) error
}
```

And we expect the underlying implementation to use a badgerDB datastore. Example:

```go
type peerStore struct {
    db datastore.Datastore
}
```

### Peer Selection

Since bootstrappers are primarily used as trustedPeers to kick off the header exchange process, we suggest to only select trustedPeers that have answered with a header in the last 24 hours. This will ensure that the node is only bootstrapping from peers that are still active.

This means initially that on node startup, specifically on store initialization, we should have a mechanism that informs us about which trustedPeers have successfully answered the `Get` request to retrieve the trusted hash (_to initialize the store_). In the current state of affairs, with `go-header` being a separate repository, we suggest to extend internal functionality of `go-header` such that the `libhead.Exchange`'s `peerTracker` is able to select peers based on the criteria mentioned above.

To achieve this, we will first extend `peerStat` to include a new attribute `isTrustedPeer`:

```go
type peerStat struct {
    // ...
    isTrustedPeer bool
}
```

and add a new method to `peerTracker` named `rewardTrustedPeer` such that it increases the score of a tracked peer (_that is a trusted peer_) by 10(_TODO: rethink this value_) points to mark it as a good _bootstrapping_ peer:

```go
func (pt *peerTracker) rewardTrustedPeer(id peer.ID) {
    pt.mu.Lock()
    defer pt.mu.Unlock()

    if stat, ok := pt.trackedPeers[id]; ok && stat.isTrustedPeer {
        stat.peerScore +=10
    }
}
```

Then, we would update `libhead.Exchange`'s private method `performRequest` to explicitly return the peers that answered with the header, then in `Get` call `rewardTrustedPeer` on the `peerTracker` when a request to a trusted peer was successful.

This will allow a tracking of which trusted peers were successful in answering the `Get` request, and thus can be used to bootstrap from.

Similarly, we will add a new method to `peerTracker` named `GetTrustedPeers` that returns a list of trusted peers that have answered with a header:

```go
func (pt *peerTracker) GetTrustedPeers() []peer.ID {
    pt.mu.Lock()
    defer pt.mu.Unlock()

    var peers []peer.ID
    for id, stat := range pt.tracedPeers {
        if stat.isTrustedPeer && stat.peerScore >= 10 {
            peers = append(peers, id)
        }
    }
    return peers
}
```

Then in `store.Init`, we pass an instance of the peer store/database mentioned above, and perform a call to this method. We then iterate over this list and add each peer to the peer store/database.

```go
// Init ensures a Store is initialized. If it is not already initialized,
// it initializes the Store by requesting the header with the given hash.
func Init[H header.Header](ctx context.Context, store header.Store[H], ex header.Exchange[H], hash header.Hash, pstore store.PeerStore) error {
    _, err := store.Head(ctx)
    switch err {
    default:
        return err
    case header.ErrNoHead:
        initial, err := ex.Get(ctx, hash)
        if err != nil {
            return err
        }

        err := store.Init(ctx, initial)
        if err != nil {
            return err
        }

        peers := ex.PeerTracker().GetTrustedPeers()
        for _, peer := range peers {
            p := ex.PeerTracker().Get(peer)
            err = pstore.Put(peer, p.Multiaddr)
            if err != nil {
                return err
            }
        }

        return err
    }
}
```

Also, we will add a routine that periodically checks for new peers that have answered with a header, and adds them to the peer store/database. This routine will be started in `store.Init`:

```go
    fx.Provide(
        // ...
        func(lc fx.Lifecycle, ex header.Exchange[H], pstore store.PeerStore) {
            lc.Append(fx.Hook{
                OnStart: func(ctx context.Context) error {
                    go func() {
                        for {
                            peers := ex.PeerTracker().GetTrustedPeers()
                            for _, peer := range peers {
                                p := ex.PeerTracker().Get(peer)
                                err = pstore.Put(peer, p.Multiaddr)
                                if err != nil {
                                    return err
                                }
                            }
                            time.Sleep(1 * time.Hour)
                        }
                    }()
                    return nil
                },
            })
        },
    )
```

(_MISSING: TODO: Add design for logic to only store peers that were active since the last 24 hours_)

### Node Startup

When the node starts up, it will first check if the `peers.db` database exists. If it does not, the node will bootstrap from the hardcoded bootstrappers. If it does, the node will bootstrap from the peers in the database.

## Status

Proposed

## Consequences

### Positive

* Allows nodes to bootstrap from previously seen peers, which allows the network to gain more decentralization.

### Negative

* peerTracker scoring logic might be impacted

<!-- 
> This section does not need to be filled in at the start of the ADR, but must be completed prior to the merging of the implementation.
>
> Here are some common questions that get answered as part of the detailed design:
>
>
> - What new data structures are needed, what data structures will be changed?
>
> - What new APIs will be needed, what APIs will be changed?
>
> - What are the efficiency considerations (time/space)?
>
> - What are the expected access patterns (load/throughput)?
>
> - Are there any logging, monitoring or observability needs?
>
> - Are there any security considerations?
>
> - Are there any privacy considerations?
>
> - How will the changes be tested?
>
> - If the change is large, how will the changes be broken up for ease of review?
>
> - Will these changes require a breaking (major) release?
>
> - Does this change require coordination with the Celestia fork of the SDK, celestia-app/-core, or any other celestiaorg repository?
 -->

## References

* [Original Issue](https://github.com/celestiaorg/celestia-node/issues/1851)
