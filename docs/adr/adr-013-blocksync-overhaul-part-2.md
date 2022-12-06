# ADR #013: Block Data Sync Overhaul: Part II - Network

## Authors

@Wondertan @renaynay

> This ADR took a while to get through all the necessary details. Special thanks to awesome @renaynay for dozens of
> deep dive calls, note-taking, thoughts, and ideas.
> Similarly, the time spent on the experiments with the format of the documents was refactored three times.

## Glossary

- LN - Light Node
- FN - Full Node
- BN - Bridge Node
- Core Network - Consensus Network of Validators/Proposers producing new Celestia Blocks
- [EDS(Extended Data Square)](https://github.com/celestiaorg/rsmt2d/blob/80d231f733e9dd8ca166c3d670470ed9a1c165d9/extendeddatasquare.go#L13) - plain Block
  data omitting headers and other block metadata
- ODS - Original Data Square or the first quadrant of the EDS. It contains real user data and padding
- Recent EDS - Newly produced EDS coming from the Core consensus network
- ND - Namespaced data/shares. Constitutes EDS

## Overview

This document is a continuation of the BlockSync Overhaul design. The previous document(TODO: Link) described the design
for the storage subsystem of the new block synchronization protocol of Celestia's Data Availability network and the Part
II focuses on networking.

The new design drastically speeds up EDS retrieval in __happy path__ by streaming ODS as a single file and recomputing
parity data forming EDS. A similar approach is taken for ND retrieval. The main goal is to make EDS and ND retrieval
consistently outpace EDS production for the max EDS size for historical and recent EDSes.

The existing EDS retrieval logic(TODO: Link something) remains a __fallback path__ if the __happy path__ fails due to a
lack of connected peers or the coordinated data withholding attack. With the __fallback path__, retrieval leverages all 
connected peers, including LNs, and granularly requests EDS pieces - __shares__, increasing the likelihood of EDS 
reconstruction while paying a slower retrieval time price.

## Decision

The decision is to design three protocols:
- [`ShrEx/EDS`](#shrexeds-protocol) - Pull-based EDS retrieval protocol

- [`ShrEx/ND`](#shrexnd-protocol) - Pull-based ND retrieval protocol

- [`ShrEx/Sub`](#shrexsub-protocol) - Push-based _notification_ protocol for the newly produced EDSes

### Key Design Decisions

- __Use [go-libp2p][go-libp2p]'s software stack to build the protocols.__ It has been extensively used in `celestia-node`
  already, proven to be robust and provides all the required components and tools for the new protocols.

- __Use [protobuf][proto] schema for protocol(TODO Link) message (de)serialization.__ Protobuf is a common 
  (de)serialization standard used in celestia-node. Even though there are better alternatives in speed, encoding
  footprint, and determinism, their benefits are negligible compared with continuing Protobuf usage for consistency and 
  simplicity.

- __FN/LNs keep existing EDS retrieval logic(TODO Link?).__ `celestia-node` already provides means to get EDS from
  the network. It is slower than block production for the EDS sizes __>128__ but has higher retrieval guarantees during
  data withholding attacks. (link)

- __FN/LN/BNs leverage implemented discovery mechanism outlined in ADR-008(TODO Link).__ The discovery in a node 
  maintains connections with FN-subnetwork. The mechanism ensures the node will receive new notifications from 
  `ShrEx/Sub` and can request data via `ShrEx/EDS` and `ShrEx/ND`.

- __FN/BNs notify connected LN/FNs when they are ready to serve a recent EDS using via [`ShrEx/Sub`](#ShrEx/Sub 
  Protocol).__ The push notification means, "I have this data; request it from me." This lazy-push approach provides 
  reliable EDS dissemination properties and is simpler than technics similar to Bitswap's want list.(TODO link)

## Protocols Specs

### ShrEx/EDS Protocol

ShrEx/EDS is a pull-based protocol with a client-server model, where LNs are clients, BNs are servers, and FNs are both. 
The protocol has only one request-response interaction followed by the ODS stream, which then reconstructs into EDS.

The protocol is designed over plain libp2p streams with `/shrex/eds/0.0.1` as the protocol ID.

#### Protobuf Schema

##### Request

```protobuf
syntax = "proto3";

message EDSRequest {
  bytes hash = 1; // identifies the requested EDS.
}
```

##### Response

```protobuf
syntax = "proto3";

enum Status {
  INVALID = 0;
  OK = 100; // data found
  NOT_FOUND = 200; // data not found
  REFUSED = 201; // request refused
}

message EDSResponse {
  Status status = 1;
}
```

#### Streaming

After flushing `EDSResponse`, a server starts streaming an ODS as a CAR file (TODO link) to a client over a libp2p 
stream.

#### Backpressure

The streaming part of the protocol does not provide any mechanics for backpressure and offloads them to the layer below.
A client-side buffer with a reasonable default size may still be added.

[Backpressure][backpressure-quic] is inherited from the QUIC. The underlying default reliable transport and stream
multiplexing protocol used in `celestia-node`.

The standard TCP transport and yamux multiplexer combination do not provide reliable backpressure on the stream level,
unlike QUIC. Therefore, TCP+yamux is not recommended and may lead to unexpected stream and connection resets in highly
congested links.

#### Flow

```
                    ┌──────┐          ┌──────┐          
                    │Client│          │Server│          
                    └──┬───┘          └──┬───┘          
                       │   Open Stream   │              
                       │ ────────────────>              
                       │                 │              
                       │    EDSRequest   │              
                       │ ────────────────>              
                       │                 │              
                       │   EDSResponse   │              
                       │ <────────────────              
                       │                 │              
                       │                 │              
          ╔══════╤═════╪═════════════════╪═════════════╗
          ║ ALT  │  STATUS:OK            │             ║
          ╟──────┘     │                 │             ║
          ║            │     CAR File    │             ║
          ║            │ <────────────────             ║
          ╚════════════╪═════════════════╪═════════════╝
                       │                 │              
                       │   Stream Close  │              
                       │ <────────────────              
                    ┌──┴───┐          ┌──┴───┐          
                    │Client│          │Server│          
                    └──────┘          └──────┘          

```

### Misbehavior

* Server sends ODS CAR file that fails EDS reconstruction.
* Client terminates stream after receiving the EDSResponse.

### ShrEx/ND Protocol

ShrEx/ND is a pull-based protocol with a client-server model, where LNs are clients and BNs/FNs are servers.
The protocol has only one request-response interaction to get all the data/shares addressed by the namespace.

The protocol is designed over plain libp2p streams with `/shrex/nd/0.0.1` as the protocol ID.

#### Protobuf Schema

##### Request

```protobuf
syntax = "proto3";

message NDRequest {
  bytes hash = 1; // identifies the EDS to get ND from
  bytes namespace = 2; // namespace of ND
}
```

##### Response

```protobuf
syntax = "proto3";

enum Status {
  INVALID = 0;
  OK = 100; // data found
  NOT_FOUND = 200; // data not found
  REFUSED = 201; // request refused
}

message Proof {
  int64 start = 1;
  int64 end = 2;
  repeated bytes Nodes = 3;
}

message Row {
  repeated bytes shares = 1;
  Proof proof = 2;
}

message NDResponse {
  Status status = 1;
  repeated Row rows = 2;
}

```

#### Streaming

The v0 version of the protocol sends ND and supporting NMT proofs as a single message. The libp2p has a max message size
of 1MB, meaning that the `NDResponse` message is limited to 1 MB.

The following versions will introduce streaming by either splitting `NDResponse` into multiple messages or by streaming
data and proofs in real-time while they are read from the disk.

#### Flow

     ┌──────┐          ┌──────┐
     │Client│          │Server│
     └──┬───┘          └──┬───┘
        │   Open Stream   │    
        │ ────────────────>    
        │                 │    
        │    NDRequest    │    
        │ ────────────────>    
        │                 │    
        │    NDResponse   │    
        │ <────────────────    
        │                 │    
        │   Stream Close  │    
        │ <────────────────    
     ┌──┴───┐          ┌──┴───┐
     │Client│          │Server│
     └──────┘          └──────┘


### ShrEx/Sub Protocol

`ShrEx/Sub` is a push-based notification protocol with the publish-subscribe model, where LNs are subscribers, BNs are 
publishers and FNs are both.

The protocol is based on libp2p's `FloodSub`(TODO Link) with `/eds-sub/0.0.1` as the topic ID.

#### Message Schema

The notification message with one field does not require serialization.
The notification is plain data hash bytes of the EDS.

#### Why not GossipSub?

In celestia-node, we extensively use libp2p's `GossipSub` router, which provides bandwidth-efficient yet secure message
dissemination over Celestia's DA p2p network. However, it does not fit well in the Recent EDS use case.

`GossipSub`'s efficacy comes from an overlay mesh network based on "physical" connections. Peers form logical links to
up to a constant DHi(12)(TODO Link) number of peers. Every gossip is __pushed__ only to these peers in the mesh
(the [fanout][gs-fanout] case is out of scope in this explanation). A new logical link is established on every new 
"physical" connection. When there are too many logical links (>DHi), random logical links are pruned. However, there is 
no differentiation between peer types so pruning can happen to any peer.

Subsequently, with `GossipSub`, any FN may prune all other BNs/FNs and end up being connected only to LNs, missing any 
new EDS notifications. Additionally, `GossipSub` implements peer exchange with pruned peers, e.g., when an FN has too 
many links, it may prune an LN and then send it a bunch of peers that are not guaranteed to be FNs. Therefore, the LN 
can end up isolated with other LNs from new EDS notifications.

The `FloodSub`, on the other hand, sends messages to every "physical" connection without overlay mesh of logical links,
which solves the problem with the cost of higher message duplication factor on the network. Although, a moderate amount 
of duplicates from different peers are helpful in this case. If the primary message sender peer does not serve data,
the senders of duplicates are requested instead.

In the future, when the network reaches a scale of ~5000 peers, we may want to design a custom PubSub router in favor of
basic`FloodSub`. The router will be based on `GossipSub` and almost untouched for the FN <-> FN interactions.
The innovation will come on the FN <-> LN front, where FNs will maintain an additional overlay mesh for connected LNs
and vise-versa. The mechanism will enable precise traffic routing control and balancing load for FNs during spikes of 
LNs activity.

PS. Besides __pushing__ gossip, GossipSub has an internal __lazy-push__ mechanism. Randomly connected peers outside
the overlay mesh are selected(GossipFactor(TODO Link)) and sent with `IHAVE` message(hash of the actual message) and can
respond with `IWANT`. In the case of an isolated FN/LN, there is a chance that it will still receive the data via the
lazy-pull mechanism; however, it is randomized, and thus the isolated node can miss notifications. We could increase 
GossipFactor to 1, which means always sending `IHAVE` to every connected peer outside the overlay mesh. However, the 
notification message is a hash, and there is no reason to pull the hash by its hash compared to a direct push of the 
hash.

## API

For the above protocols, we define the following API and components. All of them should land into `eds` pkg, as
it contains all the content related to the EDS domain. At a later point, if `eds` pkg seems bloated, we will break it
into subpkgs.

### `Client`

The EDS Client implements client-side of the `ShrEx/EDS` protocol.

#### `Client.RequestEDS`

```go
// RequestEDS requests the full EDS from one of the given peers.
//
// The peers are requested in a round-robin manner with retries until one of them gives a valid response.
// Blocks forever until the context is canceled or a valid response is given
func (c *Client) RequestEDS(context.Context, share.Root, peer.IDSlice) (rsmt2d.EDS, error)
```

### `NDClient`

The EDS Client implements client-side of the `ShrEx/NS` protocol.

#### `Client.RequestND`

```go
// RequestND requests namespaced data from one of the given peers.
//
// The peers are requested in a round-robin manner with retries until one of them gives a valid response.
// Blocks forever until the context is canceled or a valid response is given.
//
// Returns only valid data with verified inclusion against share.Root.
func (c *Client) RequestND(context.Context, share.Root, peer.IDSlice, namespace.ID) ([][]byte, error)
```

### `Server`

The EDS `Server` implements the server side of the `ShrEx/EDS` protocol. It serves `Client`s' `EDSRequest`s, responses 
with `EDSResponse` and streams ODS data from `eds.Store.GetCAR` wrapped in `ODSReader`.

`Server` may not provide any API besides constructor and lifecycle methods.

### `NDServer`

The  EDS `NDServer` is introduced to serve `NDClient`'s `NDRequest`s over the `ShrEx/ND` protocol and respond
`NDResponse`s with data and proofs coming from `eds.Store.CARBlockstore`.

`NDServer` may not provide any API besides constructor and lifecycle methods.

### `PubSub`

The EDS `PubSub` implements the `ShrEx/Sub` protocol. It enables subscribing to notifications regarding new EDSes.
It embeds a private `FloodSub` instance.

The `PubSub` follows the existing network publish-subscribe semantics in `celestia-node` for consistency and simplicity.


#### `PubSub.Broadcast`

```go
// Broadcast sends the EDS notification(data hash) to every connected peer.
func (s *PubSub) Broadcast(context.Context, datahash []byte) error
```

#### `PubSub.Subscribe`

```go
// Subscribe provides a new Subscription to EDS notifications.
func (s *PubSub) Subscribe() *Subsription

type Subscription struct {}

// Next blocks the callee until any new EDS notification(data hash) arrives.
// Returns only notifications that successfully went through the validation pipeline.
func (s *PubSub) Next(context.Context) ([]byte, error)

// Cancel cancels stops the subscription. 
func (s *PubSub) Cancel()
```

#### `PubSub.AddValidator`

```go
// Validator is an injectable func and governs EDS notification or DataHash validity.
// It receives the notification and sender peer and expects the validation result.
// Validator is allowed to block for an indefinite time or until the context is canceled.
type Validator func(context.Context, peer.ID, []byte) pubsub.ValidationResult

// AddValidator registers given Validator for EDS notifications(DataHash).
// Any amount of Validators can be registered.
func (s *PubSub) AddValidator(Validator) error
```

#### `PubSub.Close`

```go
// Close completely stops the Subscriber:
// * Unregisters all the added Validators
// * Stop the `ShrEx/Sub` topic
// * Closes the internal FloodSub instance
func (s *PubSub) Close()
```

## Integration Details

### Discovery

We decided to rely on existing discovery primitives(TODO Link). It is a single-purpose mechanism providing the API for
FN/BNs to advertise themselves, s.t. other LN/FN/BNs can discover and ensure N connections to them. However, the
current API does not let us know our discovered FN/BN peers' connections. Listing of discovered peers is required for 
`ShrEx/EDS` and `ShrEx/ND`, as their clients do not discover servers automatically and expect them to be provided.

Subsequently, we extend `discovery.Discovery` with the following method:

```go
// Peers provides a list of discovered peers in the "full" topic.
// If Discovery hasn't found any peers, it blocks until at least one peer is found.
func (d *Discovery) Peers(context.Context) ([]peer.ID, error)
```

### das.DASer

The `das.DASer` engine keeps the chain sampled and availability checked and provided. Currently, it relies on
`header.Subscribtion` to get every newly produced header from the network. Once a header is received, it can reliably 
check its availability with `share.Availability` interface. However, as we are changing the protocol, the usage of 
`share.Availability` has to be synchronized with EDS notifications from our peers via `eds.PubSub`.

To synchronize them we:
* Cache headers coming from `header.Subscribtion`, instead of calling `SharesAvailable` with `DataHash` as the key.
* Add validator to `eds.PubSub` which
  * Gets the header out of the cache by `DataHash` message body
    * If not found, waits for it within the configurable timeout
    * If nothing, rejects the message
  * Calls `SharesAvailable` with the DAH from the header and the peer the message received from

### share.Availability

Celestia-node has a central `share.Availability` interface, which guarantees a certain level of EDS data availability 
depending on the implementation. Its `SharesAvailable` method has to be extended with an additional variadic `peer.ID` 
param. Previously, the interface was generic enough and abstracted away peering details from the user. However, with the
introduction of `eds.Pubsub`, the higher level component `das.DASer` points the peers(s) to get EDSes from, and thus
peering details of one `FullAvailability` implementation has to pollute the interface.

#### FullAvailability

The `FullAvailability` implementation is responsible for checking the availability of the whole
EDS by retrieving and reconstructing it. As we are improving our EDS retrieval logic, the new ___happy path___ over
`eds.Client` integrates into the ___fallback path___ with the following flow:
* ___happy path___ EDS
  * Take peers passed to the `ShareAvailable`
    * If no peers were given, use discovered peers via `Discovery.Peers`
  * Request peers using `eds.Client`
* ___fallback path___ EDS
  * If ___happy path___ EDS operation does not finish within `BlockTime` timeout -- run `eds.Retriever` and wait until
    one of the paths finishes.
* Store EDS in `eds.Store`

Additionally, `FullAvailability` should provide `OnlyReconstruction` mode to allow testing the ___fallback path___
in isolation.

### core.Listener

The `core.Listener` is the component that exists only in BN. It listens for new blocks coming from the Core Network,
computes EDS with `ExtendedHeader`, and broadcasts it to the network. With the introduction of `ShrEx/Sub` BNs sends
notifications once it is ready to serve the new block; thus, we make `core.Listener` additionally broadcast the 
notification via `eds.PubSub`.

## Hardening

### Rate Limiting

Both `ShrEx/EDS` and `ShrEx/ND` has a server-side that serves data. In order to prevent trivial DOS vectors, rate 
limiting should be introduced. Both protocols should have a configurable amount of data requests they serve at any 
moment. If any new request exceeds the limit, the server responds with a `REFUSED` status code to the client, so that 
client can act accordingly and try another peer.

The alternative is to queue clients' requests and make the client wait until their request is processed.


## Alternative approaches

### Historical EDSes

- Extract Bitswap engine with custom transport for EDSes instead of the IPFS blocks
  - Solves both recent and historical EDS retrieval
  - Uncertain amount of efforts and thus time

### Recent EDS

- Polling connected peers
- `ExtendedHeader` gossip as acknowledgment that data is received and ready to be served.
- Implement custom engine similar to Bitswap with WANTLIST semantics(WANT_HAVE, WANT_DATA, DONT_HAVE, DONT_WANT)
  - Generic protocol that solves retrieval of historical and recent EDSes
  - Does not fit in the current time budget

[go-libp2p]: https://github.com/libp2p/go-libp2p
[proto]: https://developers.google.com/protocol-buffers
[backpressure-quic]: https://datatracker.ietf.org/doc/html/draft-ietf-quic-transport-19#section-4
[gs-fanout]: https://docs.libp2p.io/concepts/pubsub/overview/#fan-out

## ToBeRemoved/Refactored

### Recent EDS

There is a crucial edge-case for the recent data. When a Node receives a recent header it immediately goes to request the
data via `das.Daser` from the network through `share.Availability` interface. However, there is no guarantee that upon the
new `ExtendedHeader` any of the Node's peers will have the data downloaded and ready to be served. That is, gossiping
of `ExtendedHeader` outpaces fetching of EDS data.

To overcome the problem, we start using `FloodSub` router in addition to existing `GossipSub` router.

- BNs __fan-out__(TODO Link) acknowledgement messages with the DataRoot(TODO Link or specify in glossary) hashes of the
  newly produced EDS by the Core network. The acknowledgement means "I have data, request it from me".
- FNs subscribe to these messages and apply further message processing
  - Check that the received DataRoot hash is part of any recent `ExtendedHeader`
  - Request the message sender peer(other BN/FN) for the ODS committed into the DataRoot.
    - If the peer is lying or does not respond with the data it acknowledged to have - wait for acknowledges from other peers
    - After certain period of time(TODO Define) fallback to polling discovered peers in the __full node__ subnetwork.
  - Retransmit the message further to acknowledge connected peers.
- LNs subscribe to these messages either, but only check
  - LNs do not retransmit the messages because they do not serve the data and thus do not need to acknowledge that they
    have it.

### Scenarios

> This section might be deprecated. It was useful to describe the whole flow of data and used as a baseline for the
> design.

#### Scenario I - Full Node requesting an EDS

Full Node comes online, kicks of Syncer and DASer. DASer by using FullAvailability requests in parallel several
blocks. Meanwhile, FullAvailability should be in process or already discovered a few other full nodes serving EDSes.
Availability requests on the FullAvailability should be blocking until at least one FullNode is found and is connected.
Once, there is at least one, FullAvailability should send a request to the discovered FullNode in a protobuf form,
containing DataRoot of the desired EDS. The responding full node on request receival checks if the EDS exist via Has
check on the EDSStore. If so, it executes GetCAR on the EDSStore, takes the Reader likely with Readahead and writes it into the stream via
`io.CopyBuffer`, subsequently closing the stream. The buffer size should be configurable but not more than the EDS size.
It is also important to throttle EDS serving to some reasonable number of reads to limit RAM usage and mitigate a DOS vector.
This should be also be configurable for our node operators, s.t. more powerful machine could utilize their RAM resources.
Then, if there is no DataRoot yet, the request gets parked in some sort of a PubSub system to wait until the EDS gets Put
into the EDSStore. This is mainly needed for the recent blocks, as the requested node might not have an EDS at the moment of
request processing, but the EDS is almost there already

#### Scenario II - Light Node requesting an EDS

The flow is the same is in Scenario I, but without the part where the node saves the block via PutCar in the EDSStore
as it light does not need store it, but it still should be able to request it.

#### Scenario III - Light Node requesting data by namespace
