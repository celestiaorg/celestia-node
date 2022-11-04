# ADR #011: Block Data Sync Overhaul: Part II - Network

## Changelog

## Authors

@Wondertan

## Glossary

- LN - Light Node
- FN - Full Node
- BN - Bridge Node
- [EDS(Extended Data Square)](https://github.com/celestiaorg/rsmt2d/blob/master/extendeddatasquare.go#L10) - plain Block
  data omitting headers and other block metadata.
- ODS - Original Data Square or the first quadrant of the EDS. Contains real user data and padding.
- Recent EDS - Newly produced EDS coming from the Core consensus network

## Context

This document is a continuation of the BlockSync Overhaul. The previous document(TODO: Link) described design for the
storage subsystem of the new BlockSync, while the Part II focuses on the networking.

The design drastically speeds up ODS retrieval over the network. The main goal is to make block syncing faster than
block production for the max EDS size 256 for both past and recent EDSes.

## Decision and Design

The decision is to design two protocols solving the retrieval problem in two situations common in the network. 
__Recent EDS__ case solves retrieval of newly produced EDSes, while __Historical EDS__ is about every past EDS.

### Design Assumptions

- No pruning.

- All the nodes are part of the same network and bootstrapped with common set of peers.

### Key Design Decisions

- __Use protobuf schema for Protocol(TODO Link) message (de)serialization.__ Protobuf is a common (de)serialization
standard which is already used in celestia-node. Even thought there are better alternatives, their benefits are 
negligible comparing with continuing Protobuf usage for consistency and simplicity.

- __Extend implemented `eds.Retriever`(TODO Link).__ Current `eds.Retriever` API already provides means to get data from the network,
while the extension provides an efficient happy path to get the EDS in more efficient manner. Additionally, Retriever is
extended to get data by namespace.

- __Leverage implemented discovery mechanism outlined in ADR-008(TODO Link).__ Discovery allows FN/LNs to find peers in
the __full node__ subnetwork. The subnetwork consists of FNs and BNs which can consume and serve EDSes or namespaced 
data contained in those. The LNs are connected to some nodes in the subnetwork, but are considered as passive 
participants and do not serve anything.

- __BNs/FNs acknowledge connected peers when they are ready to serve a recent EDS.__ This guarantees that the recent EDS
can be reliably served by the peers.

### Historical EDS

We introduce a new simple request-based protocol with client-server model basing on the `EDSStore`(TODO:link) as a
storage backend, where LNs are clients, BNs are servers and FNs are both.

#### Protocol

The protocol is designed over plain libp2p(TODO Link) streams with `/eds-ex/0.0.1` as the protocol ID.

##### Request Protobuf Message

To request data over the wire we used protobuf message with the following structure:

```protobuf
syntax = "proto3";

message DataRequest {
  bytes dataRoot = 1;
  repeated bytes namespace = 2;
}
```

###### dataRoot

The Data Root(TODO link) identifies the EDS the data is requested from.

###### namespace

The repeated namespace field enables requesting data by namespace. This field is optional and allows requesting data for
multiple namespaces within a single EDS identified by Data Root. Empty or zero length field means getting the whole EDS.

##### Client

The client side of the equation extends the existing `eds.Retriever` component. Its `Retrieve` API is the secure way
to get the EDS via sampling the network of LNs/FNs/BNs. However, it is the main source of inefficiencies and long 
retrieval times due to the nature of the underlying Bitswap protocol and granular share retrieval. In order to solve 
this, we extend the `Retrieve` method with the new __happy path__ that fetches the EDS as a whole from FNs/BNs servers. 
The existing granular retrieval logic remains a __fallback path__, if the __happy path__ fails due to lack of connected 
FNs/BNs or the coordinated data withholding attack. With the __fallback path__ the client leverages connected LNs and 
increases likelihood of EDS reconstruction paying a slower retrieval time price.

##### Server

#### API

##### `eds.Retriever`

##### `eds.Retriever.Retrieve`

To retrieve EDS from the network, `Retrieve` method is extended with the __happy path__:

##### `eds.Reriever.RetrieveByNamespace`

To retrieve namespaced data from the network, `RetrieveByNamespace` is introduced:

### Recent EDS

There is a crucial edge-case for the recent data. When a Node receives a recent header it immediately goes to request the
data via `das.Daser` from the network through `share.Availability` interface. However, there is no guarantee that upon the
new `ExtendedHeader` any of the Node's peers will have the data downloaded and ready to be served. That is, gossiped Headers
outpace data requesting.

To overcome the problem, we introduce a new GossipSub topic where:

- BNs __fan-out__(TODO Link) the hash of the newly added Header to the network.
- FNs subscribe and during validation request the data from peers they received hashes from and only after retransmit
hashes further. This guarantees that at least one immediate peer has the data to request from.
  - If the peer is lying and does not respond with the data it acknowledged to have - its GossipSub peer score is decreased
or its get blacklisted. (TODO Decide. Blacklisting is too much?)
- LNs passively listen for new hashes, so if it requests data by namespace for the latest ExtendedHeader it can successfully get it.

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

## Alternative approaches

### Requesting EDS
- Extract Bitswap engine with custom transport for EDSes instead of the IPFS blocks
  - Solves both recent and historical EDS retrieval
  - Uncertain amount of efforts and thus time
- Implement engine similar to Bitswap with WANTLIST semantics(WANT_HAVE, WANT_DATA, DONT_HAVE, DONT_WANT)
  - Generic protocol that solves retrieval of past and recent EDSes
  - Does not fit in the current time budget

### Recent EDS

## Hardenings

## Optimizations

## Considertations

### GetSharesByNamespace

We know that LNs users will be syncing all the recent shares by a namespace, so we should provide a reliable way for 
them to dependent DataSub

