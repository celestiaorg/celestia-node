# ADR 014: Bootstrap from previous peers

## Changelog

* 2023-05-04: initial draft
* 2023-05-04: update peer selection with new simpler solution + fix language
* 2023-10-04: propose a second approach
* 2023-11-04: propose a third approach
* 2023-14-04: document decision
* 2023-25-04: update approach A to resolve security concerns

## Context

Celestia node relies on a set of peer to peer protocols that assume a decentralized network, where nodes are connected to each other and can exchange data.

Currently, when a node joins/comes up on the network, it relies, by default, on hardcoded bootstrappers for 3 things:

1. initializing its store with the first (genesis) block
2. requesting the network head to which it sets its initial sync target, and
3. bootstrapping itself with some other peers on the network which it can utilise for its operations.

This is a centralization bottleneck as it happens both during initial start-up and for any re-starts.

In this ADR, we wish to aleviate this problem by allowing nodes to bootstrap from previously seen peers. This is a simple solution that allows the network alleviate the start-up centralisation bottleneck such that nodes can join the network without the need for hardcoded bootstrappers being up on re-start

(_Original issue available in references_)

## Decision

Periodically store the addresses of previously seen peers in a key-value database, and allow the node to bootstrap from these peers on start-up, by following the methodology of:

* Approach A.

## Design

## Approach A: Hook into `peerTracker` from `libhead.Exchange` and react to its internal events

</br>

In this approach, we will rely on `peerTracker`'s scoring mechanism that tracks the fastest connected nodes, and blocks malicious ones. To make use of this functionality, we rely on the internal mechanisms of `peerTracker` for peers selection and persistence, and that is by proposing a design that allows storing `peerTracker`'s list of good peers, and opening the design space to integrate peer selection and storage logic.

## Implementation

</br>

### What systems will be affected?

* `peerTracker` and `libhead.Exchange` from `go-header`
* `newInitStore` in `nodebuilder/header.go`

### Storage

We will use a badgerDB datastore to store the addresses of previously seen good peers. The database will be stored in the `data` directory, and will have a store prefix `good_peers`. The database will have a single key-value pair, where the key is `peers` and the value is a `[]AddrInfo`

The peer store will implement the following interface:

```go
type PeerStore interface {
   // Put adds a list of peers to the store.
    Put([]peer.AddrInfo) error
    // Get returns a peer from the store.
    Load() ([]peer.AddrInfo, error)
}
```

And we expect the underlying implementation to use a badgerDB datastore. Example:

```go
type peerStore struct {
    db datastore.Datastore
}
```

Ultimately, we will use the same pattern that is used in the store implementation under `das/store.go` for the peer store.

### Peer Selection & Persistence

To periodically select the _"good peers"_ that we're connected to and store them a in "peer store", we will rely on the internals of `go-header`, specifically the `peerTracker` and `libhead.Exchange` structs.

`peerTracker` has internal logic that continuously tracks and garbage collects peers based on whether they've connected to the node, disconnected from the node, or responded to the node while taking into consideration response speed. All connected peers are kept track of in a list, and disconnected peers are marked for removal, all as a part of a garbage collection routine. `peerTracker` also blocks peers that behave maliciously, but not in a routine, instead, it does so on `blockPeer` call. Ultimately, the blocked peers will be first disconnected from before the block happens, thus the garbage collection routine will take care of cleaning them up.

We intend to use this internal logic to periodically select peers that are "good" and store them in the peer store mentioned in the storage section. To do this, we will perform the storing of the "good peers" every time `peerTracker` performs its internal garbage collection, since that's when `peerTracker` updates its peer list by removing peers marked for removal and cleans peers with low scores. (_i.e: the "bad peers" are filtered_)

To enable this, we suggest that:

1.`peerTracker` to have a reference of the `Peerstore`
2.`peerTracker` to directly call the `Peerstore`'s methods for storage

```diff
type peerTracker struct {
        // online until pruneDeadline, it will be removed and its score will be lost.
        disconnectedPeers map[peer.ID]*peerStat

+       peerstore Peerstore
+
        ctx    context.Context
        cancel context.CancelFunc
        // done is used to gracefully stop the peerTracker.
```

(_Code Snippet 1.a: Example of required changes to: go-header/p2p/peer_tracker.go_)

as explained above, we will store the new updated list on every new garbage collection cycle (_the gc interval value is configurable_):

```diff
func (p *peerTracker) gc() {
         if peer.pruneDeadline.Before(now) {
                        delete(p.disconnectedPeers, id)
                }
        }

        for id, peer := range p.trackedPeers {
                if peer.peerScore <= defaultScore {
                        delete(p.trackedPeers, id)
                }
        }
+
+       updatedPeerList := make([]peer.ID, 0, len(p.trackedPeers))
+       for _, peer := range p.trackedPeers {
+           updatedPeerList = append(updatedPeerList, PIDtoAddrInfo(peer.peerID))
+       }
+
+       p.peerstore.Put(updatedPeerList)
        p.peerLk.Unlock()
                }
        }
 }
```

(_Code Snippet 1.b: Example of required changes to: go-header/p2p/peer_tracker.go_)

The `peerTracker`'s constructor should be updated to accept the peer store:

```diff
func newPeerTracker(
        h host.Host,
        connGater *conngater.BasicConnectionGater,
+       peerstore Peerstore,
 ) *peerTracker {
```

(_Code Snippet 1.d: Example of required changes to: go-header/p2p/peer_tracker.go_)

as well as the `libhead.Exchange`'s options and construction:

```diff
 type ClientParameters struct {
        // MaxHeadersPerRangeRequest defines the max amount of headers that can be requested per 1 request.
        MaxHeadersPerRangeRequest uint64
        // RangeRequestTimeout defines a timeout after which the session will try to re-request headers
        // from another peer.
        RangeRequestTimeout time.Duration
        // TrustedPeersRequestTimeout a timeout for any request to a trusted peer.
        TrustedPeersRequestTimeout time.Duration
        // networkID is a network that will be used to create a protocol.ID
        networkID string
        // chainID is an identifier of the chain.
        chainID string
+
+       peerstore Peerstore
}
 ```

(_Code Snippet 2.a: Example of required changes to: go-header/p2p/options.go_)

 ```diff
 func NewExchange[H header.Header](
        peers peer.IDSlice,
        connGater *conngater.BasicConnectionGater,
        opts ...Option[ClientParameters],
 ) (*Exchange[H], error) {
        params := DefaultClientParameters()
        for _, opt := range opts {
                opt(&params)
        }
 
        err := params.Validate()
        if err != nil {
                return nil, err
        }
 
        ex := &Exchange[H]{
                host:       host,
                protocolID: protocolID(params.networkID),
                peerTracker: newPeerTracker(
                        host,
                        connGater,
+                      params.peerstore
                     ),
                     Params:  params,
        }
 
        ex.trustedPeers = func() peer.IDSlice {
                return shufflePeers(peers)
        }
        return ex, nil
}
 ```

(_Code Snippet 2.b: Example of required changes to: go-header/p2p/exchange.go_)

To pass the peer store to the `Exchange` (_and subsequently to the `peerTracker`_), we can rely on `WithPeerPersistence` option as follows:

```diff
+   fx.Provide(newPeerStore),
    fx.Provide(newHeaderService)
...
- func(cfg Config, network modp2p.Network) []p2p.Option[p2p.ClientParameters] {
+ func(cfg Config, network modp2p.Network, peerStore datastore.Datastore) []p2p.Option[p2p.ClientParameters] {
        return []p2p.Option[p2p.ClientParameters]{
            ...
            p2p.WithChainID(network.String()),
+           p2p.WithPeerPersistence(peerStore),
        }
    },
),
     fx.Provide(newP2PExchange(*cfg)),
```

(_Code Snippet 3.b: Example of required changes to celestia-node/nodebuilder/header/module.go_)

### Security Concerns: Long Range Attack

With the ability to retrieve previously seen good peers, we should distinguish between those and the trusted peers for cases when we have expired local sync target (_i.e: subjective head_). On such cases, we would like to strictly sync and retrieve the subjective head from our trusted peers and no other, and for other syncing tasks, include the other stored good peers. To achieve this, we need to first include information about the expiry of the local sync target:

```diff

 // newP2PExchange constructs a new Exchange for headers.
 func newP2PExchange(
        lc fx.Lifecycle,
+        storeChainHead *header.ExtendedHeader,
        bpeers modp2p.Bootstrappers,
        network modp2p.Network,
        host host.Host,
        ...
        exchange, err := p2p.NewExchange(...,
                p2p.WithParams(cfg.Client),
                p2p.WithNetworkID[p2p.ClientParameters](network.String()),
                p2p.WithChainID(network.String()),
+               p2p.WithSubjectiveInitialization(storeChainHead.IsExpired),
        )
        if err != nil {
                return nil, err
```

(_Code Snippet 3.c: Example of required changes to celestia-node/nodebuilder/header/constructors.go_)

This piece information will allow the `Exchange` to distinguish between the two cases and use the correct peer set for each case:

```diff
 func (ex *Exchange[H]) Head(ctx context.Context) (H, error) {
        log.Debug("requesting head")
+       // pool of peers to request from is determined by whether the head of the store is expired or not
+       if ex.Params.subjectiveInitialisation {
+              // if the local chain head is outside the unbonding period, only use TRUSTED PEERS for determining the head
+              peerset = ex.trustedPeers
+        } else {
+              // otherwise, node has a chain head that is NOT outdated so we can actually request random peers in addition to trusted
+              peerset = append(ex.trustedPeers, ex.params.peerstore.Get())
+        }
+
        reqCtx := ctx
        if deadline, ok := ctx.Deadline(); ok {
                // allocate 90% of caller's set deadline for requests
    ...
-   for _, from := range trustedPeers {
+         for _, from := range peerset {
                go func(from peer.ID) {
                        headers, err := ex.request(reqCtx, from, headerReq)
                        if err != nil {
```

Other side-effect changes will have to be done to methods that use `trustedPeers` restrictively for requests (_such as `performRequest`_) to allow requesting to previously seen good peers as well (_Reference: [Issue #25](https://github.com/celestiaorg/go-header/issues/25)_)
</br>

## Approach B: Periodic sampling of peers from the peerstore for "good peers" selection using liveness buckets

In this approach, we will periodically sample peers from the host's `Peerstore` and select the "good peers", with the "good peers" being the ones that have been connected to the longest possible period of time.

Taking inspiration from [this proposition](https://github.com/libp2p/go-libp2p-kad-dht/issues/254#issuecomment-633806123) we suggest to look for peers that have been continuously online, such that, for the given buckets of connection durations:

* < 1 hour ago
* < 1 day ago
* < 1 week ago
* < 1 month ago.

the "good enough peers" to select should be present in most recent buckets, and should be peers we've seen often. For example, if a peer has been connected to the node for 1 day, then it should show up in the buckets: "< 1 hour" and , "< 1 day", hence making it a "good enough peer" to select for persistence.

The bucketing logic that triages peers into the mentioned buckets is executed periodically, with a new list of "good enough peers" being selected and persisted at that same time, and also at the moment of node shutdown.

## Implementation

### Storage

We suggest to use the same storage mechanism proposed in approach A.

Note: In terms of write frequency, we intend for the periodicity of writes to be configurable.

### Peer Selection

To achieve bucket-powered peer selection, we propose to create a standalone cache like component to manage buckets, such that it tracks peers by their IDs (_`peer.ID`_) and puts them in the correct bucket every time a `Sort` method is called on the component. The component, thus, maps each peer ID to a list of timestamps, each timestamp marking when did the bucketing component see this peer.

The length of the list of timestamps will be equal to the amount of times the peer was seen, and the dates from oldest to newest will help the component sort the peer into appropriate buckets.

Example:
A list of 1 timestamp means it's been seen once, and if the timestamp was 3 hours ago, then the peer would be sorted into the "< 1 day ago" bucket.

Depending on how many peers the node will be connected to, this could be the best peer it can get, or the worst.

A better example of what a very good peer would be is:
A peer that has a list of timestamps whose length is equal to 10, and the oldest timestamp was a week ago and the newest from an hour ago. This will make the peer fall into "< 1 week ago" bucket but also all the other recent buckets.

We suggest to also turn the "amount of times seen" into buckets, such that we sort peers into buckets of "how many times" they were seen, modifying the selection criteria for good peers to become:

* Peers that fall in most recent buckets + Peers that fall in highest seen count buckets

**The interface**: We will go with the name `Bucketer` for the lack of a better one for the moment

```go
type Bucketer interface {
    AddPeer(peer.ID, time.Time)
    AddPeers([]peer.ID, time.Time)
    Sort()
    BestPeers() []peer.ID
    PurgePeer(peer.ID)
    PurgeAll()
}
```

The implementer of this interface should rely on golang maps as the native abstraction for buckets, example:

```go
type bucketer struct {
    timeBuckets map[BucketDuration][]peer.ID
    seenBuckets map[int][]peer.ID
    peersSeen map[peer.ID][]time.Time
}
```

with `BucketDuration` being a new type to enumerate all possible time buckets, example:

```go
type BucketDuration string
const (
    LTOneHour = "< 1 hour"
    LTOneDay = "< 1 day"
    ...
)
```

The `AddPeer` or `AddPeers`, and the `Sort` methods are intended to be called from within a procedure `fetchSortPersistPeers` that is called within a routine that periodically fetches peers from the peer store of the host (_`peerstore.Peerstore`_) which contains all the peers we are connected to.

`fetchSortPersistPeers` will also be called on node shutdown.

`fetchSortPersistPeers` will be as follows:

```go
func fetchSortPersistPeers(bcktr Bucketer, store PeerStore, host host.Host) {
    goodPeers := make([]peer.PeerInfo, 0)
    peers := host.Peerstore().PeersWithAddrs()
    bcktr.AddPeers(peers)
    bcktr.Sort()
    bestPeers = bcktr.BestPeers()
    for _, bestpeer := range bestPeers {
        goodPeers = append(goodPeers, host.Peerstore().PeerInfo(bestpeer))
    }
    store.Put(goodPeers)
}
```

### Node Startup

In node startup, we propose to use the same mechanism that is proposed in approach A, but also add changes to register the `fetchSortPersistPeers` routine to trigger periodically every other `filterBootstrappersInterval`.

```diff
    baseComponents = fx.Options(
        ...
        fx.Provide(newModule),
+       fx.Invoke(func (ctx context.Context, lc fx.Lifecycle, cfg Config, bcktr Bucketer, pstore PeerStore, host host.Host) {
+         ch := make(chan struct{})
+         ticker := time.NewTicker(cfg.filterBootstrappersInterval * time.Millisecond)
+         go func() {
+           select {
+             case <-ctx.Done():
+             case <-ch:
+             case <-ticker.C:
+                fetchSortPersistPeers(bcktr, pstore, host) 
+          }
+
+          lc.Append(fx.Hook{
+             onStop: func(ctx context.Context) error) {
+               ch <- struct{}{}
+             },
+          })
+   })
),

```

(_Code Snippet 5.a: Example of required changes to celestia-node/nodebuilder/p2p/module.go_)

## Approach C: Periodic sampling of peers from the peerstore for randomized "good peers" selection

In this approach, we will periodically sample peers from the hosts's `Peerstore` and select random peers from the peer store, similar to approach B, only without the bucketing logic. We suggest to do away with the bucketing logic as it will introduce maintenance costs, and will likely end up in go-libp2p-kad-dht sooner than we anticipate.

Thus, approach C consist basically of approach B minus the bucketing logic.

## Implementation

### Storage

We suggest to use the same storage mechanism proposed in approach A.

### Peer Selection

Assuming a routine `fetchPersistPeers` to be called periodically (_period to be defined_)
and on node shutdown, this routine should:

1. Fetch all peers with their addresses available in the address book (_from the peer store_)
2. Randomly select a number (_to be defined_) from the fetched peer list
     2.a If the fetched peer list's length is less than the required number, we simply select all available peers, as there wouldn't be enough peers to perform randomized selection
3. Upsert the new list in place of the old one.

```go
func fetchPersistPeers(store PeerStore, host host.Host) {
    goodPeers := make([]peer.PeerInfo, 0)
    peers := host.Peerstore().PeersWithAddrs()
    bestPeers := getRandomPeers(peers)
    for _, bestpeer := range bestPeers {
        goodPeers = append(goodPeers, host.Peerstore().PeerInfo(bestpeer))
    }
    store.Put(goodPeers)
}
```

_The selection of random peers is done through a `getRandomPeers` function that returns a slice of randomly selected peers from an original slice_

This will result in periodic updates to the stored list of "good peers" on disk. The same should happen on node shutdown.

If the node list does not change, then we will be upserting the same list again which should cause no problems.

### Node Startup

In node startup, we propose to use the same mechanism that is proposed in approach A.

## Other explored solutions

### On-Disk Peerstore from libp2p

Upon evaluating [the on-disk implementation of the peerstore](https://github.com/libp2p/go-libp2p/blob/master/p2p/host/peerstore/pstoreds/peerstore.go) from libp2p, we did not find that it will bring value to the current endeavor of this ADR, as the on-disk implementation simply moves peer store operations to disk and nothing more.

We were hoping that we would be able to fully rely on it such that if the node shuts down and boots up again, it can boot up with a peer store full of peers and addresses, however, experiment  shows that although we successfully boot up with a list of peer IDs, most addresses are not stored, because they were expired before the node shut down, thus not persisted.

With that, we need an additional mechanism that stores addresses long past their TTL and judges when a peer is good enough to be stored as an ephemeral bootstrapper.

Both suggested approaches, A and B, do not conflict with using an on-disk peer storage, however switching to an on-disk one would be either due to memory usage optimizations or special requirement to store the peer store on disk, and not necessarily because the on-disk peer store provides special functionality.

## Status

Accepted

## Consequences

### Positive

* Allows nodes to bootstrap from previously seen peers, which allows the network to gain more decentralization.

## References

* [Original Issue](https://github.com/celestiaorg/celestia-node/issues/1851)
