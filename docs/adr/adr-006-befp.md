# ADR #006: Celestia-Node Bad encoding fraud proof(BEFP)

## Changelog

- 2022.03.03 - init commit
- 2022.03.08 - added pub-sub

## Authors

@vgonkivs @Bidon15 @adlerjohn @Wondertan

## Context

In the case where a Full Node receives `ErrByzantineRow`/`ErrByzantineCol` from the [rsmt2d](https://github.com/celestiaorg/rsmt2d) library, it generates a fraud proof and broadcasts it to Light Nodes such that the Light Nodes are notified that the corresponding block could be malicious.

## Decision

Started disscussion within:

- https://github.com/celestiaorg/celestia-node/issues/4
- https://github.com/celestiaorg/celestia-node/issues/263

## Detailed Design
A fraud proof is generated if recovered data does not match with its respective row/column roots during block repair process

The result of RepairExtendedDataSquare will be an error [ErrByzantineRow](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L18-L22)/[ErrByzantineCol](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L28-L32):

- Both errors consist of 
  - row/column numbers that do not match with the Merkle root
  - shares that were successfully repaired and verified (all correct shares).

`ErrByzantineRow`/`ErrByzantineCol` are returned from `RetrevieData`. In addition, `das.Daser`:

1. Generates a MerkleProofs for respective verified shares
2. Creates a BEFP
3. Broadcasts it(BEFP) to light nodes via separate sub-service.

`das.Daser` imports a data structure that implements `proof.Broadcaster` interface that uses libp2p.pubsub under the hood:

```go
// Broadcaster is a generic interface that sends a different kinds of fraud proofs to all subscribed on particular topic nodes
type Broadcaster interface {
   // Broadcast takes a Fraud proof data stucture that implements standart BinaryMarshal interface and sends data to light nodes using libp2p pub-sub under the hood.
   Broadcast(ctx context.Context, p encoding.BinaryMarshaller)  
}
```

Data serialization/deserialization will be performed with `protobuf.Marshal`/`protobuf.Unmarshal` and data structure will be described in proto file:

```proto3
enum FraudProofType {
   BadEncoding=0;
}

message MerkleProof {
   repeated bytes MerkleProof = 1;
}

message BadEnconding {
   required FraudProofType Type = 1;
   required uint64 Height = 2;
   repeated bytes Shares = 3;
   repeated MerkleProof MerkleProofs = 4;
}
```

From the other side, light nodes have the ability to subscribe to a particular fraud proof update and verify received data:

```go
// Subscriber encompasses the behavior necessary to
// subscribe/unsubscribe from new FraudProofs events from the
// network.
type Subscriber interface {
   // Subscribe allows to subscribe on pub sub topic by it's type
   Subscribe(ctx context.Context, proofType pb.FraudProofType) (Subscription, error)
}
```

```go
type Subscription interface {
   NextProof() (encoding.BinaryUnmarshal, error)
}
```

```go
type BadEncoding struct {
   Height uint64
   Shares [][]byte
   MerkleProofs [][][]byte
}
```

```go
type FraudService struct {
   pubsub *pubsub.PubSub
   topic  map[string]*pubsub.Topic
}

func(f *FraudService) Subscribe(ctx context.Context, proofType pb.FraudProofType) (Subscription, error){}

func(f *FraudService) Broadcast(ctx context.Context, p encoding.BinaryMarshaller){}
```

## Status
Proposed

## Consequences

### Positive

Detect incorrectly encoded block

## References

Data Availability(Bad Encoding) Fraud Proofs: [#4](https://github.com/celestiaorg/celestia-node/issues/4)
   
Implement stubs for BadEncodingFraudProofs: [#263](https://github.com/celestiaorg/celestia-node/issues/263) 
