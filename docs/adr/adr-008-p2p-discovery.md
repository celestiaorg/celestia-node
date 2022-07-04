# ADR #008: Celestia-Node Full Node Discovery

## Changelog

- 2022.07.04 - discovery data structure

## Authors

@vgonkivs

## Context

This ADR is intended to describe p2p full node discovery in celestia node. 
P2P discovery helps for light and full nodes to find other full nodes on the network at specified topic(`full`).
As soon as full node is found and connection is established with it, then it will be added to an internal store.
## Decision

- https://github.com/celestiaorg/celestia-node/issues/599

## Detailed design
```go
// discovery combines advertise and discover services and allows to store discovered nodes.
type discovery struct {
	// storage where all discovered and active peers are saved.
	set  *limitedSet
	// libp2p.Host allows to connect to discovered peers.
	host host.Host
	// libp2p.Discovery allows to advertise and discover peers.
	disc core.Discovery
}

// discoverer used to protect light nodes of being advertised as they support only peer discovery.
type discoverer interface {
    findPeers(ctx context.Context)
}
```
### Full Nodes behavior:
1. Node starts advertising itself over DHT at `full` namespace after the system boots up in order to be found.
2. Node starts finding other full nodes to be able to join the Full Node network.
3. As soon as a new peer is found, the node will try to establish a connection with it. In case the connection is successful
the node will Tag Peer and add to the internal storage.


### Light Nodes behavior:
1. Node starts finding full nodes over DHT at "full" namespace using the `discoverer` interface.
2. As soon as a new peer is found, the node will try to establish a connection with it. In case the connection is successful
   the node will Tag Peer and add to the internal storage.

Tagging allows to protect peer from being killed by ConnManager.

![discovery](https://user-images.githubusercontent.com/40579846/177183260-92d1c390-928b-4c06-9516-24afea94d1f1.png)

## Status
Proposed