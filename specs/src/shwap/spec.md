# Shwap Protocol Specification

## Terms and Definitions

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
client, server and node.

_**Client**_: The Peer that requests content by content identifies over Shwap.

_**Server**_: The Peer that responds with content over Shwap.

_**Node**_: The peer that namespacesis both the client and the server.

_**Proof**_: Merkle inclusion proof of the data in the DataSquare.

## Rationale

### Multihashes and CID

Shwap takes inspiration from content addressability, but breaks-free from hash-based only model to optimize message sizes
and data request patterns. In some way, it hacks into multihash abstraction to make it contain data that isn't in fact a
hash. Furthermore, the protocol does not include hash digests in the multihashes. The authentication of the messages
happens using externally provided data commitment.

## Protocol Dependencies

### Bitswap

Shwap depends on Bitswap for swapping bits in fully distributed p2p-manner.

## Share Identifiers

This section defines list of supported share identifiers. Share identifiers defined by Shwap can be used to uniquely
identify any [share container](#share-containers) over a chain with arbitrary number of [DataSquares][square], like a range of 
[shares][shares], a row or a [blob][blob]. Every share identifier relates to a respective share container and wise-versa.

Identifiers are embeddable to narrow down to the needed content. (TODO: Describe better)

Identifiers MUST have a fixed size for their fields. Subsequently, protobuf can't be used for CID serialization due to
varint usage. Instead, identifiers use simple binary big endian serialization.

Table of supported identifiers with their respective multihash and codec codes. This table is supposed to be extended
whenever any new identifier is added.

| Name     | Multihash | Codec  |
|----------|-----------|--------|
| RowID    | 0x7811    | 0x7810 |
| SampleID | 0x7801    | 0x7800 |
| DataID   | 0x7821    | 0x7820 |

### RowID

RowID identifies the [Row shares container](#row-container) in a [DataSquare][square].

RowID identifiers are formatted as shown below:

```
RowID {
    Height: u64;
    RowIndex: u16;
}
```

The fields with validity rules that form RowID are:

**Height**: A uint64 representing the chain height with the data square. It MUST be bigger than zero. 

**RowIndex**: An uint16 representing row index points to a particular row. It MUST not exceed the number of Row roots in
[DAH][dah].

Serialized RowID MUST have length of 10 bytes.

### SampleID

SampleID identifies a Sample container of a single share in a [DataSquare][square].

SampleID identifiers are formatted as shown below:
```
SampleID {
    RowID;
    ShareIndex: u16; 
}
```

The fields with validity rules that form SampleID are:

[**RowID**](#rowid): A RowID of the sample. It MUST follow [RowID](#rowid) formatting and field validity rules.

**ShareIndex**: A uint16 representing the index of the sampled share in the row. It MUST not exceed the number of Column
roots in [DAH][dah].

Serialized SampleID MUST have length of 12 bytes.

### DataID

DataID identifies [namespace][ns] Data container of shares within a _single_ Row. That is, namespace shares spanning 
over multiple Rows are identified with multiple identifiers.

DataID identifiers are formatted as shown below:
```
DataID {
    RowID;
    Namespace;
}
```

The fields with validity rules that form DataID are:

[**RowID**](#rowid): A RowID of the namespace data. It MUST follow [RowID](#rowid) formatting and field validity rules.

[**Namespace**][ns]: A fixed-size bytes array representing the Namespace of interest. It MUST follow [Namespace][ns] 
formatting and its validity rules.

Serialized DataID MUST have length of 39 bytes.

## Share Containers

This section defines list of supported share containers. Share containers encapsulate a set of data shares with [DAH][dah]
inclusion proof. Share containers are identified by [share identifiers](#share-identifiers).

### Row Container

Row containers encapsulate Row of the [DataSquare][square].

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

**RowHalf**: A two-dimensional variable size byte arrays representing left half of shares in the row. It MUST be equal 
to the number of Columns roots in [DAH][dah] divided by two. These shares MUST only be from the left half of the row. 
The right half is computed using Leopard GF16 Reed-Solomon erasure-coding. Afterward, the [NMT][nmt] is built over both 
halves and the computed NMT root MUST be equal to the respective Row root in [DAH][dah].

### Sample Container

Sample containers encapsulate single shares of the [DataSquare][square].

Sample containers are protobuf formatted using the following proto3 schema:
```protobuf
syntax = "proto3";

message Sample {
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
and be verified against the respective root from Row or Column axis in [DAH][dah]. The axis is defined by ProofType field.

**ProofType**: An enum defining which root the Proof is coming from. It MUST be either RowProofType or ColumnProofType. 

### Data Container

Data containers encapsulate user submitted data under [namespaces][ns].

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
and be verified against the respective root from Row or Column axis in [DAH][dah]. The axis is defined by ProofType field.

Namespace data may span over multiple rows in which case all the data is encapsulated in multiple containers. This is 
done

## Protocol Extensions

This section is a placeholder for future protocol extensions like new new identifiers and containers.

## Considerations

### Bitswap CID integration

The naive question would be: "Why not to make content verification after Bitswap provided it back over its API?"
Intuitively, this would simplify a lot and wouldn't require "hacking" CID. However, this has an important downside - 
the Bitswap in such case would consider the content as fetched and valid, sending DONT_WANT message to its peers, while
the message might be invalid according to the verification rules.

[square]: https://celestiaorg.github.io/celestia-app/specs/data_structures.html#2d-reed-solomon-encoding-scheme
[shares]: https://celestiaorg.github.io/celestia-app/specs/shares.html#abstract
[shares-format]: https://celestiaorg.github.io/celestia-app/specs/shares.html#share-format
[blob]: https://celestiaorg.github.io/celestia-app/specs/data_square_layout.html#blob-share-commitment-rules
[dah]: https://celestiaorg.github.io/celestia-app/specs/data_structures.html#availabledataheader
[ns]: https://celestiaorg.github.io/celestia-app/specs/namespace.html#abstract
[nmt]: https://github.com/celestiaorg/nmt/blob/master/docs/spec/nmt.md
[nmt-pb]: https://github.com/celestiaorg/nmt/blob/f5556676429118db8eeb5fc396a2c75ab12b5f20/pb/proof.proto
[nmt-verify]: https://github.com/celestiaorg/nmt/blob/master/docs/spec/nmt.md#namespace-proof-verification