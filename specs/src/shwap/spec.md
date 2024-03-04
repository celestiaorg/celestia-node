# Shwap Protocol Specification

## Abstract

This document specifies Shwap - the simple and expressive yet extensible and future-proof messaging framework aiming to
solve critical inefficiencies and standardize messaging of Celestia's Data Availability p2p network.

Shwap defines a messaging framework to be exchanged around the DA p2p network in a trust-minimized way without enforcing
transport(QUIC/TCP or IP) or application layer protocol semantics(e.g., HTTP/x). Using this framework, Shwap
declares the most common messages and provides options for stacking them with lower-level protocols.
Shwap can be stacked together with application protocol like HTTP/x, [KadDHT][kaddht], [Bitswap][bitswap] or any custom
protocol.

## Motivation

The current Data Availability Sampling (DAS) network protocol is inefficient. A _single_ sample operation takes log2(k)
network roundtrips (where k is the square size). This is not practical and does not scale for the theoretically unlimited
data square that the Celestia network enables. The main motive here is a protocol with O(1) roundtrip for _multiple_
samples, preserving the assumption of having 1/n honest peers connected.

Initially, Bitswap and IPLD were adopted as the basis for the DA network protocols, including DAS,
block synchronization (BS), and blob/namespace data retrieval (ND). They gave battle-tested protocols and tooling with
pluggability to rapidly scaffold Celestia's DA network. However, it came with the price of scalability limits and
roundtrips, resulting in slower BS than block production. Before the network launch, the transition
to the optimized [ShrEx protocol][shrex] for BS and integrating [CAR and DAGStore-based storage][storage] happened
optimizing BS and ND. However, DAS was left untouched, preserving its weak scalability and roundtrip inefficiency.

Shwap messaging stacked together with Bitswap protocol directly addresses described inefficiency and provides a foundation
for efficient communication for BS, ND, and beyond.

## Rationale

The atomic primitive of Celestia's DA network is the share. Shwap standardizes messaging and serialization for shares.
Shares are grouped together, forming more complex data types(Rows, Blobs, etc.). These data types are encapsulated in
containers, e.g., Row container groups shares of a particular row. Containers can be identified with the share identifiers
in order to request, advertise or index the containers. The combination of containers and identifiers provides an extensible
and expressive messaging framework for groups of shares and enables efficient single roundtrip request-response
communication.

Many share groups or containers are known in the Celestia network, and systemizing this is the main reason behind setting
up this simple messaging framework. A single place with all the possible Celestia DA messages must be defined, which node
software and protocol researchers can rely on and coordinate. Besides, this framework is designed to be
future-proof and sustain changes in the core protocol's data structures and proving system as long shares stay the
de facto atomic data type.

Besides, there needs to be systematization and a joint knowledge base with all the edge cases for possible protocol
compositions of Shwap with lower-level protocols Bitswap, KadDHT, or Shrex, which Shwap aims to describe.

## Specification

### Terms and Definitions

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
"SHOULD", "SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and
"OPTIONAL" in this document are to be interpreted as described in BCP
14 [RFC2119] [RFC8174] when, and only when, they appear in all
capitals, as shown here.

Commonly used terms in this document are described below.

_**Shwap**_: The protocol described by this document. Shwap is a
portmanteau name of words share and swap.

_**[Share][shares]**_: The core data structure of DataSquare **"swapped"** between peers.

_**[DataSquare][square]**_: The DA square format used by Celestia DA network.

_**[DAH][dah]**_: The Data Availability Header with Row and Column commitments.

_**[Namespace][ns]**_: The namespace grouping sets of shares.

_**Peer**_: An entity that can participate in a Shwap protocol. There are three types of peers:
client, server, and node.

_**Client**_: The Peer that requests content by content identifies over Shwap.

_**Server**_: The Peer that responds with content over Shwap.

_**Node**_: The peer that is both the client and the server.

_**Proof**_: A Merkle inclusion proof of the data in the DataSquare.

### Message Framework

This section defines Shwap's messaging framework. Every group of shares that needs to be exchanged over the network
MUST define its [share identifier](#share-identifiers) and [share container](#share-containers), as well as, follow
their described rules.

#### Share Identifiers

Identifiers MUST have a fixed size for their fields. Subsequently, protobuf SHOULD NOT be used for CID serialization due
to varints and lack of fixed size arrays. Instead, identifiers use simple binary big endian serialization.

Identifiers MAY embed each other to narrow down the scope of needed shares. For example, [SampleID](#sampleid) embeds
[RowID](#rowid) as every sample lay on a particular row.

#### Share Containers

Share containers encapsulate a set of data shares with [DAH][dah] inclusion proof. Share containers are identified by
[share identifiers](#share-identifiers).

#### Versioning

If a defined share container or identifier requires an incompatible change, the new message type MAY be introduced
suffixed with a new major version starting from v1. E.g., if the Row message needs a revision, RowV1 is created.

### Messages

This section defines all the supported Shwap messages, including share identifiers and containers. All the new
future messages should be described in the section.

#### RowID

RowID identifies the [Row shares container](#row-container) in a [DataSquare][square].

RowID identifiers are formatted as shown below:

```text
RowID {
    Height: u64;
    RowIndex: u16;
}
```

The fields with validity rules that form RowID are:

**Height**: A uint64 representing the chain height with the data square. It MUST be bigger than zero.

**RowIndex**: An uint16 representing row index points to a particular row. It MUST not exceed the number of Row roots in
[DAH][dah].

Serialized RowID MUST have a length of 10 bytes.

#### Row Container

Row containers are protobuf formatted using the following proto3 schema:

```protobuf
syntax = "proto3";

message Row {
  bytes row_id = 1;
  repeated bytes row_half = 2;
}
```

The fields with validity rules that form Row containers are:

[**RowID**](#rowid): A RowID of the Row Container. It MUST follow [RowID](#rowid) formatting and field validity rules.

**RowHalf**: A two-dimensional variable size byte arrays representing left half of shares in the row. It MUST equal the number of Columns roots in [DAH][dah] divided by two. These shares MUST only be from the left half of the row.
The right half is computed using Leopard GF16 Reed-Solomon erasure-coding. Afterward, the [NMT][nmt] is built over both
halves and the computed NMT root MUST be equal to the respective Row root in [DAH][dah].

#### SampleID

SampleID identifies a Sample container of a single share in a [DataSquare][square].

SampleID identifiers are formatted as shown below:

```text
SampleID {
    RowID;
    ColumnIndex: u16; 
}
```

The fields with validity rules that form SampleID are:

[**RowID**](#rowid): A RowID of the sample. It MUST follow [RowID](#rowid) formatting and field validity rules.

**ColumnIndex**: A uint16 representing the column index of the sampled share; in other words, the share index in the row. It
MUST stay within the number of Column roots in [DAH][dah].

Serialized SampleID MUST have a length of 12 bytes.

#### Sample Container

Sample containers encapsulate single shares of the [DataSquare][square].

Sample containers are protobuf formatted using the following proto3 schema:

```protobuf
syntax = "proto3";

message sample {
    bytes sample_id = 1;
    bytes sample_share = 2;
    Proof sample_proof = 3;
    ProofType proof_type = 4;
}

enum ProofType {
  RowProofType = 0;
  ColProofType = 1;
}
```

The fields with validity rules that form Sample containers are:

[**SampleID**](#sampleid): A SampleID of the Sample container. It MUST follow [SampleID](#sampleid) formatting and field
validity rules.

**SampleShare**: A variable size array representing the share contained in the sample. Each share MUST follow [share
formatting and validity][shares-format] rules.

**Proof**: A [protobuf formated][nmt-pb] [NMT][nmt] proof of share inclusion. It MUST follow [NMT proof verification][nmt-verify]
and be verified against the respective root from the Row or Column axis in [DAH][dah]. The axis is defined by the ProofType field.

**ProofType**: An enum defining which axis root the Proof is coming from. It MUST be either RowProofType or ColumnProofType.

#### DataID

DataID identifies [namespace][ns] Data container of shares within a _single_ Row. That is, namespace shares spanning
over multiple Rows are identified with multiple identifiers.

DataID identifiers are formatted as shown below:

```text
DataID {
    RowID;
    Namespace;
}
```

The fields with validity rules that form DataID are:

[**RowID**](#rowid): A RowID of the namespace data. It MUST follow [RowID](#rowid) formatting and field validity rules.

[**Namespace**][ns]: A fixed-size bytes array representing the Namespace of interest. It MUST follow [Namespace][ns]
formatting and its validity rules.

Serialized DataID MUST have a length of 39 bytes.

#### Data Container

Data containers encapsulate user-submitted data under [namespaces][ns].

Data containers are protobuf formatted using the following proto3 schema:

```protobuf
syntax = "proto3";

message Data {
    bytes data_id = 1;
    repeated bytes data_shares = 2;
    Proof data_proof = 3;
}
```

The fields with validity rules that form Data containers are:

[**DataID**](#dataid): A DataID of the Data container. It MUST follow [DataID](#dataid) formatting and field validity
rules.

**DataShares**: A two-dimensional variable size byte arrays representing left data shares of a namespace in the row.
Each share MUST follow [share formatting and validity][shares-format] rules.

**Proof**: A [protobuf formated][nmt-pb] [NMT][nmt] proof of share inclusion. It MUST follow [NMT proof verification][nmt-verify]
and be verified against the respective root from the Row or Column axis in [DAH][dah]. The axis is defined by the ProofType field.

Namespace data may span over multiple rows, in which case all the data is encapsulated in multiple containers. This is
done

## Protocol Compositions

This section specifies compositions of Shwap with other protocols. While Shwap is transport agnostic, there are rough
edges on the protocol integration, which every composition specification has to describe.

### Bitswap

[Bitswap][bitswap] is an application-level protocol for sharing verifiable data across peer-to-peer networks.
Bitswap operates as a dynamic want-list exchange among peers in a network. Peers continuously update and share their
want lists of desired data in real time. It is promptly fetched if at least one connected peer has the needed data.
This ongoing exchange ensures that as soon as any peer acquires the sought-after data, it can instantly share it with
those in need.

Shwap is designed to be synergetic with Bitswap, as that is the primary composition to be deployed in Celestia's DA
network. Bitswap provides the 1/N peers guarantee and can parallelize fetching across multiple peers. Both of these properties
significantly contribute to Celestia's efficient DAS protocol.

Bitswap runs over the libp2p stack, which provides QUIC transport integration. Subsequently, Shwap will benefit from features
libp2p provides together with transport protocol advancements introduced in QUIC.

#### Multihashes and CID

Bitswap is tightly coupled with Multihash and CID notions, establishing the [content addressability property][content-address].
Bitswap operates over Blocks of data that are addressed and verified by CIDs. Based on that, Shwap integrates into
Bitswap by complying with both of these interfaces. The [Share Containers](#share-containers) are Blocks that are identified
via [Share Identifiers](#share-identifiers).

Even though Shwap takes inspiration from content addressability, it breaks free from the hash-based model to optimize
message sizes and data request patterns. In some way, it hacks into multihash abstraction to make it contain data that
is not, in fact, a hash. Furthermore, the protocol does not include hash digests in the multihashes. The authentication of
the messages happens using externally provided data commitment.

This creates a bunch of complexities with the [reference Golang implementation][gimpl] that are necessary if forking
and substantially diverging the upstream is not an option. The naive question would be: "Why not make content
verification after Bitswap provided it back over its API?" Intuitively, this would simplify much and would not require
"hacking" CID. However, this has an important downside - the Bitswap, in such a case, would consider the request finalized
and the content as fetched and valid, sending a DONT_WANT message to its peers. In contrast, the message might still be invalid
according to the verification rules.

However, Bitswap still requires multihashes and CID codecs to be registered. Therefore, we provide a table for the
supported [share identifiers](#share-identifiers) with their respective multihash and CID codec codes. This table
should be extended whenever any new share identifier is added.

| Name     | Multihash | Codec  |
|----------|-----------|--------|
| RowID    | 0x7811    | 0x7810 |
| SampleID | 0x7801    | 0x7800 |
| DataID   | 0x7821    | 0x7820 |

## Backwards Compatibility

Swap is incompatible with the old sampling protocol.

After rigorous investigation, the celestia-node team decided against _implementing_ backward compatibility with
the old protocol into the node client due to the immense complications it brings. Instead, the simple and time-efficient
strategy is transiently deploying infrastructure for old and new versions, allowing network participants to migrate
gradually to the latest version. We will first deprecate the old version, and once the majority has migrated, we will
terminate the old infrastructure.

## Considerations

### Security

Shwap does not change the security model of Celestia's Data Availability network and changes the underlying
protocol for data retrieval.

Essentially, the network and its codebase get simplified and require less code and infrastructure to operate. This in turn
decreases the amount of implementation vulnerabilities, DOS vectors, message amplification, and resource exhaustion attacks.
However, new bugs may be introduced, as with any new protocol.

### Protobuf Serialization

Protobuf is a widely adopted serialization format and is used within Celestia's protocols. This was quite an obvious choice
for consistency reasons, even though we could choose other more efficient and advanced formats like Cap'n Proto.

## Reference Implementation

- [Go reference implementation with Bitswap composition][gimpl]
- [Rust implementation with Bitswap composition][rimpl]

[shrex]: https://github.com/celestiaorg/celestia-node/blob/0abd16bbb05bf3016595498844a588ef55c63d2d/docs/adr/adr-013-blocksync-overhaul-part-2.md
[storage]: https://github.com/celestiaorg/celestia-node/blob/a33c80e20da684d656c7213580be7878bcd27cf4/docs/adr/adr-011-blocksync-overhaul-part-1.md
[bitswap]: https://docs.ipfs.tech/concepts/bitswap/
[content-address]: https://fission.codes/blog/content-addressing-what-it-is-and-how-it-works/
[kaddht]: https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf
[square]: https://celestiaorg.github.io/celestia-app/specs/data_structures.html#2d-reed-solomon-encoding-scheme
[shares]: https://celestiaorg.github.io/celestia-app/specs/shares.html#abstract
[shares-format]: https://celestiaorg.github.io/celestia-app/specs/shares.html#share-format
[dah]: https://celestiaorg.github.io/celestia-app/specs/data_structures.html#availabledataheader
[ns]: https://celestiaorg.github.io/celestia-app/specs/namespace.html#abstract
[nmt]: https://github.com/celestiaorg/nmt/blob/master/docs/spec/nmt.md
[nmt-pb]: https://github.com/celestiaorg/nmt/blob/f5556676429118db8eeb5fc396a2c75ab12b5f20/pb/proof.proto
[nmt-verify]: https://github.com/celestiaorg/nmt/blob/master/docs/spec/nmt.md#namespace-proof-verification
[gimpl]: https://github.com/celestiaorg/celestia-node/pull/2675
[rimpl]: https://github.com/eigerco/lumina/blob/561640072114fa5c4ed807e94882473476a41dda/node/src/p2p/shwap.rs
