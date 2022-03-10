# ADR #006: Celestia-Node Bad encoding fraud proof(BEFP)

## Changelog

- 2022.03.03 - init commit
- 2022.03.08 - added pub-sub

## Authors

@vgonkivs @Bidon15 @adlerjohn @Wondertan

## Context

In case when a Full Node receives `ErrByzantineRow`/`ErrByzantineCol` from the [rsmt2d](https://github.com/celestiaorg/rsmt2d) library, it generates a fraud proof.  After generation of it(fraud proof), a Full Node is broadcasting the fraud proof to Light Nodes, so they(Light Nodes) are notified that a block could be malicious.

## Decision

Started disscussion within:

- https://github.com/celestiaorg/celestia-node/issues/4
- https://github.com/celestiaorg/celestia-node/issues/263

## Detailed Design
It should be generated when after repairing the entire block, we detect recovered data does not match with its respective row/column roots. The result of RepairExtendedDataSquare will be an error [ErrByzantineRow](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L18-L22)/[ErrByzantineCol](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L28-L32):

Both of these errors consist of row/column numbers that do not match with the Merkle root and shares that were successfully repaired and verified (all correct shares).

`ErrByzantineRow`/`ErrByzantineCol` should be returned from RetrevieData. In this case `das.Daser` should generate a MerkleProofs for respective verified shares and create a BEFP. Then broadcast it to light clients via separate sub-service.

`das.Daser` should import a data structure that implements `proof.Broadcaster` interface that uses libp2p.pubsub under the hood:

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

From the other side, the light nodes should be able to subscribe on particular fraud proof updates and verify received data:

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
   mu    sync.Mutex
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
