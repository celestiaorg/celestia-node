# ADR 014: Bootstrap from previous peers

## Changelog

* 2023-05-04: initial draft
* 2023-05-04: update peer selection with new simpler solution + fix language
* 2023-10-04: propose a second approach
* 2023-11-04: propose a third approach

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

## Design

## Approach A: Hook into `peerTracker` from `libhead.Exchange` and react to its internal events

</br>

In this approach, we will track on two important internal events from `peerTracker` in `libhead.Exchange` for bootstrapper peers selection and persistence purposes, and that is by proposing a design that allows access to `peerTracker`'s list of good peers and bad peers, and opening the design space to integrate peer selection and storage logic.

## Implementation

</br>

### What systems will be affected?

* `peerTracker` and `libhead.Exchange` from `go-header`
* `newInitStore` in `nodebuilder/header.go`

### Storage

We will use a badgerDB datastore to store the addresses of previously seen peers. The database will be stored in the `data` directory, and will be named `good_peers`. The database will have a single key-value pair, where the key is `peers` and the value is a `[]PeerInfo`

The peer store will implement the following interface:

```go
type PeerStore interface {
   // Put adds a list of peers to the store.
    Put([]peer.AddrInfo) error
    // Get returns a peer from the store.
    Load() ([]peer.AddrInfo, error)
    // Delete removes a peer from the store.
    Delete(peer.ID) error
}
```

\*: _Delete will be used in in the special case where we store a peer and then it gets blocked by `peerTracker`_

And we expect the underlying implementation to use a badgerDB datastore. Example:

```go
type peerStore struct {
    db datastore.Datastore
}
```

### Peer Selection & Persistence

To periodically select the _"good peers"_ that we're connected to and store them in the peer store, we will rely on the internals of `go-header`, specifically the `peerTracker` and `libhead.Exchange` structs.

`peerTracker` has internal logic that continuiously tracks and garbage collects peers, so that if new peers connect to the node, peer tracker keeps track of them in an internal list, as well as it marks disconnected peers for removal. `peerTracker` also blocks peers that behave maliciously.

We would like to use this internal logic to periodically select peers that are "good" and store them in the peer store. To do this, we will "hook" into the internals of `peerTracker` and `libhead.Exchange` by defining new "event handlers" intended to handle two main events from `peerTracker`:

1. `OnUpdatedPeers`
2. `OnBlockedPeer`

_**Example of required changes to: go-header/p2p/peer_tracker.go**_

```diff
type peerTracker struct {
        // online until pruneDeadline, it will be removed and its score will be lost.
        disconnectedPeers map[peer.ID]*peerStat
 
+       onUpdatedPeers func([]peer.AddrInfo)
+       onBlockedPeer  func(peer.AddrInfo)
+
        ctx    context.Context
        cancel context.CancelFunc
        // done is used to gracefully stop the peerTracker.
```

`OnUpdatedPeers` will be called whenever `peerTracker` updates its internal list of peers on its `gc` routine, which updates every 30 minutes (_value is configurable_):

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
        p.peerLk.Unlock()
+
+       p.onUpdatedPeers(updatedPeerList)
                }
        }
 }
```

where as `OnBlockedPeer` will be called whenever `peerTracker` blocks a peer through its method `blockPeer`.

```diff
 // blockPeer blocks a peer on the networking level and removes it from the local cache.
 func (p *peerTracker) blockPeer(pID peer.ID, reason error) {
        // add peer to the blacklist, so we can't connect to it in the future.
        err := p.connGater.BlockPeer(pID)
        if err != nil {
                log.Errorw("header/p2p: blocking peer failed", "pID", pID, "err", err)
        }
        // close connections to peer.
        err = p.host.Network().ClosePeer(pID)
        if err != nil {
                log.Errorw("header/p2p: closing connection with peer failed", "pID", pID, "err", err)
        }
 
        log.Warnw("header/p2p: blocked peer", "pID", pID, "reason", reason)
 
        p.peerLk.Lock()
        defer p.peerLk.Unlock()
        // remove peer from cache.
        delete(p.trackedPeers, pID)
        delete(p.disconnectedPeers, pID)
+       p.onBlockedPeer(PIDToAddrInfo(pID))
 }
```

The `PIDToAddrInfo` converts a `peer.ID` to a `peer.AddrInfo` by retrieving such info from the host's peer store (_`host.Peerstore().PeerInfo(peerId)`_).

The `peerTracker`'s constructor should be updated to accept these new event handlers:

```diff
 type peerTracker struct {
 func newPeerTracker(
        h host.Host,
        connGater *conngater.BasicConnectionGater,
+       onUpdatedPeers func([]peer.ID),
+       onBlockedPeer func(peer.ID),
 ) *peerTracker {
```

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
+       OnUpdatedPeers func([]peer.AddrInfo)
+       OnBlockedPeer func(peer.AddrInfo)
 }
 ```

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
+                       params.OnUpdatedPeers
+                       params.OnBlockedPeer,
                ),
                Params: params,
        }
 
        ex.trustedPeers = func() peer.IDSlice {
                return shufflePeers(peers)
        }
        return ex, nil
 }
 ```

And then define `libhead.Exchange` options to set the event handlers on the `peerTracker`:
_**Example of required changes to: go-header/p2p/go-header/p2p/options.go**_

```go
func WithOnUpdatedPeers(f func([]]peer.ID)) Option[ClientParameters] {
       return func(p *ClientParameters) {
               switch t := any(p).(type) { //nolint:gocritic
               case *PeerTrackerParameters:
                       p.OnUpdatedPeers = f
               }
       }
}

func WithOnBlockedPeer(f func([]]peer.ID)) Option[ClientParameters] {
       return func(p *ClientParameters) {
               switch t := any(p).(type) { //nolint:gocritic
               case *PeerTrackerParameters:
                       p.OnBlockedPeer = f
               }
       }
}
```

The event handlers to be supplied are callbacks that either:

1. Put the new peer list into the peer store
2. Remove a blocked peer from the peer store

Such callbacks are easily passable as options at header module construction time, example:

```go
// newP2PExchange constructs a new Exchange for headers.
func newPeerStore(cfg Config) func(ds datastore.Datastore) (PeerStore, error) {
       return PeerStore(namespace.Wrap(ds, datastore.NewKey("peer_list")))
}
```

```diff
+++ celestia-node/nodebuilder/header/module.go
+   fx.Provide(newPeerStore),
    fx.Provide(newHeaderService)

...

- func(cfg Config, network modp2p.Network) []p2p.Option[p2p.ClientParameters] {
+ func(cfg Config, network modp2p.Network, peerStore datastore.Datastore) []p2p.Option[p2p.ClientParameters] {
        return []p2p.Option[p2p.ClientParameters]{
            ...
            p2p.WithChainID(network.String()),
+           p2p.WithOnUpdatedPeers(func(peers []peer.ID) {
+               topPeers := getTopPeers(peers, cfg.BootstrappersLimit)
+               peerStore.Put(datastore.NewKey("topPeers"), topPeers)
+           }),
        }
    },
),
                        fx.Provide(newP2PExchange(*cfg)),
```

*:_for requests are that are performed through `session.go`_

### Node Startup

When the node starts up, it will first check if the `peers` database exists. If it does not, the node will bootstrap from the hardcoded bootstrappers. If it does, the node will bootstrap from the peers in the database.

_**Example of required changes to: celestia-node/nodebuilder/header/constructors.go**_

```diff
 // newP2PExchange constructs a new Exchange for headers.
 func newP2PExchange(cfg Config) func(
        fx.Lifecycle,
        modp2p.Bootstrappers,
        modp2p.Network,
        host.Host,
        *conngater.BasicConnectionGater,
        []p2p.Option[p2p.ClientParameters],
+       pstore peerstore.Peerstore,
 ) (libhead.Exchange[*header.ExtendedHeader], error) {
        return func(
                lc fx.Lifecycle,
                bpeers modp2p.Bootstrappers,
                network modp2p.Network,
                host host.Host,
                conngater *conngater.BasicConnectionGater,
                opts []p2p.Option[p2p.ClientParameters],
        ) (libhead.Exchange[*header.ExtendedHeader], error) {
                peers, err := cfg.trustedPeers(bpeers)
                if err != nil {
                        return nil, err
                }
+              list, err := pstore.Load()
+               if err != nil {
+                       return nil, err
+               }
+               peers := append(peers, list...)
                ids := make([]peer.ID, len(peers))
                for index, peer := range peers {
                        ids[index] = peer.ID
                        host.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
                }
                exchange, err := p2p.NewExchange[*header.ExtendedHeader](host, ids, conngater, opts...)
                if err != nil {
                        return nil, err
                }
                lc.Append(fx.Hook{
                        OnStart: func(ctx context.Context) error {
                                return exchange.Start(ctx)
                        },
                        OnStop: func(ctx context.Context) error {
                                return exchange.Stop(ctx)
                        },
                })
                return exchange, nil
        }
 }
```

</br>

## Approach B: Periodic sampling of peers from the peerstore for "good peers" selection using liveness buckets

In this approach, we will periodically sample peers from the peerstore and select the "good peers", with the "good peers" being the ones that have been connected to the longest possible amount of time.

Taking inspiration from [this proposition](https://github.com/libp2p/go-libp2p-kad-dht/issues/254#issuecomment-633806123) we suggest to look for peers that have been continuously online, such that, for the given buckets of connection durations:

* < 1 hour ago
* < 1 day ago
* < 1 week ago
* < 1 month ago.

The "good enough peers" to select should be present in most recent buckets, and should be peers we've seen often. For example, if a peer has been connected to the node for 1 day, then it should figure up in the buckets: "< 1 hour" and , "< 1 day", hence making it a "good enough peer" to select for persistence.

The bucketing logic that triages peers into the mentioned buckets is executed periodically, with a new list of "good enough peers" being selected and persisted also periodically, and at the moment of node shutdown.

## Implementation

### Storage

We suggest to use the same storage mechanism proposed in approach A.

### Peer Selection

To achieve bucket-powered peer selection, we propose to create a standalone cache like component to manage buckets, such that it tracks peers by their IDs (_`peer.ID`_) and puts them in the correct bucket every time a `Sort` method is called on the component. The component, thus, maps each peer ID to a list of timestamps, each timestamp marking when did the bucketing component see this peer.

The length of the list of timestamps will be equal to the amount of times the peer was seen, and the dates from oldest to newest will help the component sort the peer into appropriate buckets.

Example:
A list of 1 timestamp means it's been seen once, and if the timestamp was 3 hours ago, then the peer would be sorted into the "< 1 day ago" bucket.

Depending on how many peers the node will be connected to, this could be the best peer it can get, or the worst.

A better example of what a very good peer would be is:
A peer with list of timestamps whose length is equal to 10, and the oldest timestamp was a week  ago. This will make the peer fall into "< 1 week ago" bucket.

We suggest to also turn the "amount of times seen" into buckets, such that we sorted peers into buckets of "how many times" they were seen, modifying the selection criteria for good peers to become:

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

### Node Startup

In node startup, we propose to use the same mechanism that is proposed in approach A.

## Approach C: Periodic sampling of peers from the peerstore for randomized "good peers" selection

In this approach, we will periodically sample peers from the peer store and select random peers from the peer store, similar to approach B, only without the bucketing logic. We suggest to do away with the bucketing logic as it will introduce maintenance costs, and will likely end up in go-libp2p-kad-dht sooner than we anticipate.

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

This will result in periodic updates of the stored list of "good peers" on disk. The same should happen on node shutdown.

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

Proposed

## Consequences

### Positive

* Allows nodes to bootstrap from previously seen peers, which allows the network to gain more decentralization.

## References

* [Original Issue](https://github.com/celestiaorg/celestia-node/issues/1851)
