# ADR 013: Bootstrap from previous peers

## Changelog

* 2023-05-04: initial draft
* 2023-05-05: update peer selection with new simpler solution + fix language

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
   // Put adds a list of peers to the store.
    Put([]peer.AddrInfo) error
    // Get returns a peer from the store.
    Load() ([]peer.AddrInfo, error)
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

### Peer Selection & Persistence

To periodically select the _"good peers"_ that we're connected to and store them in the peer store, we will rely on the internals of `go-header`, specifically the `peerTracker` and `libhead.Exchange` structs.

`peerTracker` has internal logic that continuiously tracks and garbage collects peers, so that if new peers connect to the node, peer tracker keeps track of them in an internal list, as well as it marks disconnected peers for removal. `peerTracker` also blocks peers that fail to answer*.

We would like to use this internal logic to periodically select peers that are "good" and store them in the peer store. To do this, we will "hook" into the internals of `peerTracker` and `libhead.Exchange` by defining new "event handlers" intended to handle two main events from `peerTracker`:

1. `OnUpdatedPeers`
2. `OnBlockedPeer`

```diff
+++ go-header/p2p/peer_tracker.go
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
+++ go-header/p2p/peer_tracker.go
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

where are `OnBlockedPeer` will be called whenever `peerTracker` blocks a peer through its method `blockPeer`.

```diff
+++ go-header/p2p/peer_tracker.go
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
We are assuming a function named `PIDToAddrInfo` that converts a `peer.ID` to a `peer.AddrInfo` struct for this example's purpose. 

The `peerTracker`'s constructor should be updated to accept these new event handlers:
```diff
+++ go-header/p2p/peer_tracker.go
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
+++ go-header/p2p/options.go
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
+       onUpdatedPeers func([]peer.AddrInfo)
+       onBlockedPeer func(peer.AddrInfo)
 }
 ```

 ```diff
+++ go-header/p2p/exchange.go
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
```go
+++ go-header/p2p/options.go
func WithOnUpdatedPeers(f func([]]peer.ID)) Option[ClientParameters] {
       return func(p *ClientParameters) {
               switch t := any(p).(type) { //nolint:gocritic
               case *PeerTrackerParameters:
                       p.onUpdatedPeers = f
               }
       }
}

func WithOnBlockedPeer(f func([]]peer.ID)) Option[ClientParameters] {
       return func(p *ClientParameters) {
               switch t := any(p).(type) { //nolint:gocritic
               case *PeerTrackerParameters:
                       p.onBlockedPeer = f
               }
       }
}
```

The event handlers to be supplied are callbacks that either:

1. Put the new peer list into the peer store
2. Remove a blocked peer from the peer store

Such callbacks are easily passable as options sat header module construction time, example:

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

When the node starts up, it will first check if the `peers.db` database exists. If it does not, the node will bootstrap from the hardcoded bootstrappers. If it does, the node will bootstrap from the peers in the database.

```diff
+++ celestia-node/nodebuilder/header/constructors.go
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

## Status

Proposed

## Consequences

### Positive

* Allows nodes to bootstrap from previously seen peers, which allows the network to gain more decentralization.


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
