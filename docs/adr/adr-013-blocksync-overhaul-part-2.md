# ADR #011: Block Data Sync Overhaul: Part II - Network

## Authors

@Wondertan @renenay

> This ADR took a while to get through all the necessary details. Special thanks to awesome @renenay for dozens of
> deep dive call, note-taking, thoughts and ideas.
> Similarly, the time was spent on the experiments with the format of the documents was refactored 3 times.

## Glossary

- LN - Light Node
- FN - Full Node
- BN - Bridge Node
- Core Network - Consensus Network of Validators/Proposers producing new Celestia Blocks
- [EDS(Extended Data Square)](https://github.com/celestiaorg/rsmt2d/blob/master/extendeddatasquare.go#L10) - plain Block
  data omitting headers and other block metadata
- ODS - Original Data Square or the first quadrant of the EDS. Contains real user data and padding
- Recent EDS - Newly produced EDS coming from the Core consensus network
- ND - Namespaced data/shares. Constitutes EDS

## Overview

This document is a continuation of the BlockSync Overhaul design. The previous document(TODO: Link) described design for the
storage subsystem of the new block synchronization protocol of Celestia's Data Availability network, while the Part II
focuses on the networking.

The new design drastically speeds up EDS retrieval in __happy path__ by streaming ODS as a single file and recomputing
parity data forming EDS. Similar approach is taken for ND retrieval. The main goal is to make EDS and ND retrieval
consistently outpace EDS production for the max EDS size for historical and recent EDSes.

The existing EDS retrieval logic(TODO: Link something) remains a __fallback path__, if the __happy path__ fails due to lack of connected
peers or the coordinated data withholding attack. With the __fallback path__ retrieval leverages all connected peers,
including LNs, and granulary requests EDS pieces - __shares__, increasing the likelihood of EDS reconstruction, while
paying a slower retrieval time price.

## Decision

The decision is to design three protocols:
- [`ShrEx/EDS`](#ShrEx/EDS Protocol) - Pull-based EDS retrieval protocol
- 
- [`ShrEx/ND`](#ShrEx/ND Protocol) - Pull-based ND retrieval protocol
- 
- [`ShrEx/Sub`](#ShrEx/Sub Protocol) - Push-based _notification_ protocol for the newly produced EDSes

### Key Design Decisions

- __Use [go-libp2p][go-libp2p]'s software stack to build the protocols.__ It's extensively used in `celestia-node` already,
proven to be robust and provides all the requirRed components and tools for the new protocols.

- __Use [protobuf][proto] schema for protocol(TODO Link) message (de)serialization.__ Protobuf is a common (de)serialization
standard which is already used in celestia-node. Even thought there are better alternatives, their benefits are
negligible comparing with continuing Protobuf usage for consistency and simplicity.

- __FN/LNs keep existing EDS retrieval logic(TODO Link?).__ `celestia-node` already provides means to get EDS from
the network. It is slower than block production for the EDS sizes __>128__, but has higher retrieval guarantees during
data withholding attack. (link)

- __FN/LN/BNs leverage implemented discovery mechanism outlined in ADR-008(TODO Link).__ The discovery in a node maintains connections
with FN-subnetwork. This ensures the node will receive new notifications from `ShrEx/Sub` and can request data via
`ShrEx/EDS` and `ShrEx/ND`.

- __FN/BNs notify connected LN/FNs when they are ready to serve a recent EDS using via [`ShrEx/Sub`](#ShrEx/Sub Protocol).__
This lazy-push approach provides reliable EDS dissemination properties and turns out to be simpler than technics similar
to Bitswap's wantlist.(TODO link)

## Protocols Specs

### ShrEx/EDS Protocol

ShrEx/EDS is a pull-based protocol with client-server model, where LNs are clients, BNs are servers
and FNs are both. The protocol has only one request-response interaction followed by EDS stream.

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
  OK = 100; // data found
  NOT_FOUND = 200; // data not found
  REFUSED = 201; // request refused
}

message EDSResponse {
  Status status = 1;
}
```

#### Streaming

After flushing `EDSResponse`, server starts streaming EDS file to client over the libp2p stream.

#### Backpressure

The streaming part of the protocol does not provide any mechanics for backpressure and offloads them to layer below.
Client-side buffer with reasonable default size may still be added.

[Backpressure][backpressure-quic] is inherited from the QUIC. The underlying default reliable transport and stream
multiplexing protocol used in `celestia-node`.

The common TCP transport and yamux multiplexer combination, does not provide reliable backpressure on stream level,
unlike QUIC. Therefore, using TCP+yamux is not recommended and may lead to unexpected stream and connections resets in case of highly
congested links.

#### Flow

// TODO Make a simple sequence diagram

### ShrEx/ND Protocol

ShrEx/ND is a pull-based protocol with client-server model, where LNs are clients and BNs/FNs are servers.
The protocol has only one request-response interaction to get all the data/shares addressed by namespace

The protocol is designed over plain libp2p streams with `/shrex/eds/0.0.1` as the protocol ID.

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
  OK = 100; // data found
  NOT_FOUND = 200; // data not found
  REFUSED = 201; // request refused
}

message NDResponse {
  Status status = 1;
  bytes  data = 2;
  bytes  proof = 3;
}
```

#### Streaming

The v0 version of the protocol sends ND and supporting NMT proofs as a single message. The libp2p has a max message size
of 1MB, meaning that the `NDResponse` message is limited with 1MB.

The following versions will introduce streaming by either splitting `NDResponse` into multiple messages or by streaming
data and proofs in real time, while they are read from the disk.

#### Flow

// TODO Make a simple sequence diagram

### ShrEx/Sub Protocol

`ShrEx/Sub` is push-based notification protocol with PubSub model, where LNs are subscribers, BNs are publishers and
FNs are both.

The protocol is based on libp2p's `FloodSub`(TODO Link) with `/ShrEx/Sub/0.0.1` as topic ID.

#### Message Schema

The notification message with one field does not require serialization.
The notification is plain data hash bytes of the EDS.

#### Why not GossipSub?

In celestia-node we use libp2p's `GossipSub` router extensively, which provides bandwidth efficient yet secure way of message
dissemination over the Celestia's DA p2p network. However, it does not fit well in the Recent EDS use case.

`GossipSub`'s efficacy comes from overlay mesh network based over "physical" connections. Peers form logical links to
up to constant DHi(12)(TODO Link) number of peers. Every gossiped message goes only to these peers in the mesh. A new
logical link is established on every new "physical" connection. When there are too many logical links (>DHi), random
logical links are pruned. However, there is no differentiation between peer types, so pruning can happen to any peer.

Subsequently, with `GossipSub` any FN may prune all other BNs/FNs and end up being connected only to LNs, missing any new
EDS notifications. Additionally, `GossipSub` implements peer exchange with pruned peers, e.g. when a FN has too many links,
it may prune a LN and then send it a bunch of peers that are not guaranteed to be FNs. Therefore, the LN can similarly end
up isolated with other LNs with no ability to listen for the new EDS notifications.

The `FloodSub` on the other hand sends messages to every "physical" connection without overlay mesh of logical links,
which solves the problem with the cost of higher message duplication factor on the network. Although, moderate amount of
duplicates coming from different peers are useful in this case. If the primary message sender peer does not serve data,
the senders of duplicates are requested instead.

In the future when the network reaches scale of ~5000 peers, we may want to design custom PubSub router in favour of basic
`FloodSub`. The router will be based on `GossipSub` and almost kept entirely untouched for the FN <-> FN interactions.
The innovation will come on the FN <-> LN front, where FNs will maintain an additional overlay mesh for connected LNs
and vise-versa. This will enable precise traffic routing control and balancing load for FNs during spikes of LNs activity.

## API

For the above protocols we define the following API and components. All them should land into `eds` pkg, as
tt contains all the content related to the EDS domain.

### `Client`

The EDS Client implements client-side of the `ShrEx/EDS` protocol.

#### `Client.RequestEDS`

```go
// RequestEDS requests the full EDS from one of the given peers.
//
// The peers are requested in round-robin manner with retries until one of them gives a valid response.
// Blocks forever until the context is canceled and/or valid response is given.
func (c *Client) RequestEDS(context.Context, share.Root, peer.IDSlice) (rsmt2d.EDS, error)
```

### `NDClient`

The EDS Client implements client-side of the `ShrEx/NS` protocol.

#### `Client.RequestND`

```go
// RequestND requests namespaced data from one of the given peers.
//
// The peers are requested in round-robin manner with retries until one of them gives a valid response.
// Blocks forever until the context is canceled and/or valid response is given.
func (c *Client) RequestND(context.Context, share.Root, peer.IDSlice, namespace.ID) ([][]byte, error)
```

### `Server`

The EDS `Server` implements server side of `ShrEx/EDS` protocol. It serves `Client`s' `EDSRequest`s, responses with
`EDSResponse` and streams ODS data coming from `eds.Store.GetCAR` wrapped in `ODSReader`.

`Server` may not provide any API.

### `NDServer`

The  EDS `NDServer` is introduced to serve `NDClient`'s `NDRequest`s over `ShrEx/ND` protocol and respond
`NDResponse`s with data and proofs coming from `eds.Store.Blockstore`.

`NDServer` may not provide any API.

### `PubSub`

The EDS `PubSub` implements `ShrEx/Sub` protocol. It enables subscribing for notifications regarding new EDSes.
It embeds a private `FloodSub` instance.

The `PubSub` follows the existing network pubsubbing semantics in `celestia-node` for consistency and simplicity.

#### `PubSub.Broadcast`

```go
// Broadcast sends the EDS notification(data hash) to every connected peer.
func (s *Subscriber) Broadcast(context.Context, datahash []byte) error
```

#### `PubSub.Subscribe`

```go
// Subscribe provides a new Subscription for EDS notifications.
func (s *Subscriber) Subscribe() *Subsription

type Subscription struct {}

// Next blocks the callee until any new EDS notification(data hash) arrives.
// Returns only notifications which successfully went through validation pipeline.
func (s *Subscription) Next(context.Context) ([]byte, error)

// Cancel cancels stops the subscription. 
func (s *Subscription) Cancel()
```

#### `PubSub.AddValidator`

```go
// Validator is an injectable func and governs EDS notification or datahash validity.
// It receives the notification and sender peer and expects the result of the validation.
// Validator is allowed to be blocking for indefinite time or until the context is canceled.
type Validator func(context.Context, peer.ID, []byte) pubsub.ValidationResult

// AddValidator registers given Validator for EDS notifications(datahash).
// Any amount of Validators can be registered.
func (s *Subscriber) AddValidator(Validator) error
```

#### `PubSub.Close`

```go
// Close completely stops the Subscriber:
// * Unregisters all the added Validators
// * Stop the `ShrEx/Sub` topic
// * Closes the internal FloodSub instance
func (s *Subscriber) Close()
```

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
