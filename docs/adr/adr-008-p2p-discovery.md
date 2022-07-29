# ADR #008: Celestia-Node Full Node Discovery

## Changelog

- 2022.07.04 - discovery data structure
- 2022.07.20 - detail bridge node behavior (@renaynay, @Wondertan)

## Authors

@vgonkivs

## Context

This ADR is intended to describe p2p full node discovery in celestia node.
P2P discovery helps light and full nodes to find other full nodes on the network at the specified topic(`full`).
As soon as a full node is found and connection is established with it, then it(full node) will be added to a set of peers(limitedSet).

## Decision

- <https://github.com/celestiaorg/celestia-node/issues/599>

## Detailed design

```go
// peersLimit is max amount of peers that will be discovered.
peersLimit = 3

// discovery combines advertise and discover services and allows to store discovered nodes.
type discovery struct {
    // storage where all discovered and active peers are saved.
    set  *limitedSet
    // libp2p.Host allows to connect to discovered peers.
    host host.Host
    // libp2p.Discovery allows to advertise and discover peers.
    disc core.Discovery
}

// limitedSet is a thread safe set of peers with given limit.
// Inspired by libp2p peer.Set but extended with Remove method.
type limitedSet struct {
    lk sync.RWMutex
    ps map[peer.ID]struct{}

    limit int
}
```

### Full Nodes behavior

1. A Node starts advertising itself over DHT at `full` namespace after the system boots up in order to be found.
2. A Node starts finding other full nodes, so it can be able to join the Full Node network.
3. As soon as a new peer is found, the node will try to establish a connection with it. In case the connection is successful
the node will call [Tag Peer](https://github.com/libp2p/go-libp2p-core/blob/525a0b13017263bde889a3295fa2e4212d7af8c5/connmgr/manager.go#L35) and add peer to the peer set, otherwise discovered peer will be dropped.

### Bridge Nodes behavior

Bridge nodes will behave as full nodes, both advertising themselves at the `full` namespace and also actively finding/connecting to other full nodes.
Bridge nodes do not perform sampling in order to reconstruct blocks since they get block data directly from the core network, but in order to increase the probability of a favourable network topology where `full` nodes are connected to enough nodes that provide enough shares in order to repair an EDS, bridge nodes should also actively connect to other nodes that advertise themselves as `full`.
For example, while unlikely, it is possible that full nodes can be partitioned from bridge nodes such that they receive a header via gossipsub and try to reconstruct via sampling over that header, but are not connected to enough peers with enough shares to repair the EDS.
Bridge node active discovery can alleviate this potential edge case by increasing the likelihood of full node <-> bridge node connections such that a wider variety of shares are available to full node such that they can repair the EDS.

### Light Nodes behavior

1. A node starts finding full nodes over DHT at `full` namespace using the `discoverer` interface.
2. As soon as a new peer is found, the node will try to establish a connection with it. In case the connection is successful
   the node will call [Tag Peer](https://github.com/libp2p/go-libp2p-core/blob/525a0b13017263bde889a3295fa2e4212d7af8c5/connmgr/manager.go#L35) and add peer to the peer set, otherwise discovered peer will be dropped.

Tagging protects connections from ConnManager trimming/GCing.

```go
// peerWeight is a weight of discovered peers.
// peerWeight is a number that will be assigned to all discovered full nodes,
// so ConnManager will not break a connection with them.
peerWeight = 1000
```

![discovery](https://user-images.githubusercontent.com/40579846/177183260-92d1c390-928b-4c06-9516-24afea94d1f1.png)

## Status

Merged
