# ADR #006: Celestia-Node Bad encoding fraud proof(BEFP)

## Changelog

- 2022.03.03 - init commit
- 2022.03.08 - added pub-sub

## Authors

@vgonkivs @Bidon15 @adlerjohn @Wondertan @renaynay
 
## Context

In the case where a Full Node receives `ErrByzantineRow`/`ErrByzantineCol` from the [rsmt2d](https://github.com/celestiaorg/rsmt2d) library, it generates a fraud proof and broadcasts it to Light Nodes such that the Light Nodes are notified that the corresponding block could be malicious.

## Decision

BEFPs are addressed in the two below issues:

- https://github.com/celestiaorg/celestia-node/issues/4
- https://github.com/celestiaorg/celestia-node/issues/263

## Detailed Design
A fraud proof is generated if recovered data does not match with its respective row/column roots during block reparation. 

The result of RepairExtendedDataSquare will be an error [ErrByzantineRow](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L18-L22)/[ErrByzantineCol](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L28-L32):

- Both errors consist of 
  - row/column numbers that do not match with the Merkle root
  - shares that were successfully repaired and verified (all correct shares).

`ErrByzantineRow`/`ErrByzantineCol` are returned as the error value from `RetrieveData`. 

In addition, `das.Daser`:

1. Generates a MerkleProofs for respective verified shares from [nmt](https://github.com/celestiaorg/nmt/blob/master/nmt.go) tree
2. Creates a BEFP
3. Broadcasts it(BEFP) to light nodes via separate sub-service.

`das.Daser` imports a data structure that implements `proof.Broadcaster` interface that uses libp2p.pubsub under the hood:


```go
type FraudProofType string

const (
   BadEncoding FraudProofType = "BadEncoding"
)

type Proof interface {
   Height() (uint64, error)
   MerkleProofs() ([][][]byte, error)
   ValidateBasic(*da.DataAvailabilityHeader) error

   // NOTE: should we add?
   // add encoding.BinaryUnmarshaller
   Payload() ([]byte, error)
}
```

```go
// FraudNotifier is a generic interface that sends a different kinds of fraud proofs to all subscribed on particular topic nodes
type FraudNotifier interface {
   // Broadcast takes a Fraud proof data stucture that implements standart BinaryMarshal interface and sends data to light nodes using libp2p pub-sub under the hood.
   Notify(ctx context.Context, p Proof)  
}
```

Data serialization/deserialization will be performed with `protobuf.Marshal`/`protobuf.Unmarshal` and data structure will be described in proto file:

```proto3
message MerkleProof {
   repeated bytes MerkleProof = 1;
}

message BadEnconding {
   required string Type = 1;
   required uint64 Height = 2;
   repeated bytes Shares = 3;
   repeated MerkleProof MerkleProofs = 4;
   uint8 Position = 5;
   bool isRow = 6;
}
```

From the other side, light nodes have the ability to subscribe to a particular fraud proof update and verify received data:

```go
// Subscriber encompasses the behavior necessary to
// subscribe/unsubscribe from new FraudProofs events from the
// network.
type Subscriber interface {
   // Subscribe allows to subscribe on pub sub topic by it's type
   Subscribe(ctx context.Context, proofType FraudProofType) (Subscription, error)
}
```

```go
type Subscription interface {
   Proof() (Proof, error)
}

type FraudSub struct {
   pubsub *pubsub.PubSub 
}

func NewFraudSub(p *pubsub.PubSub)(Subscription, error){}
func(s *FraudSub) Proof() (Proof, error){}
```

```go
type BadEncoding struct {
   Height uint64
   Shares [][]byte
   MerkleProofs []nmt.Proof
   Position uint8
   isRow bool
}
```

```go
// NOTE: re-think how FraudService should be designed(and contructed) for full nodes and for light nodes
type FraudService struct {
   badEncodingSub Subscription
   fraudNotifier FraudNotifier
   // @Wondertan please take a look at Proof interface
   // It will return a codec and a payload of the message
   //codec map[FraudProofType] fpFunc
}

func(f *FraudService) Subscribe(ctx context.Context, proofType FraudProofType) (Subscription, error){}

func(f *FraudService) Notify(ctx context.Context, p Proof){}
```

## Status
Proposed

## References

Data Availability(Bad Encoding) Fraud Proofs: [#4](https://github.com/celestiaorg/celestia-node/issues/4)
   
Implement stubs for BadEncodingFraudProofs: [#263](https://github.com/celestiaorg/celestia-node/issues/263) 
