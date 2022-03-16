# ADR #006: Celestia-Node Fraud Service

## Changelog

- 2022.03.03 - init commit
- 2022.03.08 - added pub-sub
- 2022.03.15 - added BEFP verification

## Authors

@vgonkivs @Bidon15 @adlerjohn @Wondertan @renaynay
 
## 1. Bad Encoding Fraud Proof (BEFP)
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

Base on `ErrByzantineRow`/`ErrByzantineCol` internal fields we should generate a [MerkleProofs](https://github.com/celestiaorg/nmt/blob/master/proof.go#L17) for respective verified shares from [nmt](https://github.com/celestiaorg/nmt/blob/master/nmt.go) tree return as the `ErrBadEncoding` from `RetrieveData`. 

```go
type ErrBadEncoding struct {
   Shares []*Share
   Position uint8
   isRow bool
}

type Share struct {
   Share []byte
   Proof nmt.Proof
}
```

In addition, `das.Daser`:

1. Creates a BEFP:

```go
const (
   BadEncoding FraudProofType = "BadEncoding"
)

type BadEncoding struct {
   Type FraudProofType
   Height uint64
   Shares []*Share
   Position uint8
   isRow bool
}
```

2. Notify all light nodes via separate sub-service via proto message:

```proto3

message MerkleProof {
  int64          total     = 1;
  int64          index     = 2;
  bytes          leaf_hash = 3;
  repeated bytes aunts     = 4;
}

message Share {
   bytes Share = 1;
   MerkleProof Proof = 2;
}

message BadEnconding {
   required string Type = 1;
   required uint64 Height = 2;
   repeated Share Shares = 3;
   uint8 Position = 4;
   bool isRow = 5;
}
```
Data serialization/deserialization will be performed with `protobuf.Marshal`/`protobuf.Unmarshal`.

`das.Daser` imports a data structure that implements `proof.FraudNotifier` interface that uses libp2p.pubsub under the hood:

```go
// FraudNotifier is a generic interface that sends a different kinds of fraud proofs to all subscribed on particular topic nodes
type FraudNotifier interface {
   // Broadcast takes a Fraud proof data stucture that implements standart BinaryMarshal interface and sends data to light nodes using libp2p pub-sub under the hood.
   Notify(ctx context.Context, p Proof)  
}
```

```go
// FraudProofType is a enum type that represents particular fraud proof
type FraudProofType string

// Proof is a generic interface that will be used for all types of fraud proofs
type Proof interface {
   Type() FraudProofType
   Height() (uint64, error)
   Validate(*header.ExtendedHeader) error

   encoding.BinaryMarshaller
   encoding.BinaryUnmarshaller
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
// Subscription provides only valid proofs, because the validation is done under the hood of pub-sub
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
// NOTE: re-think how FraudService should be designed(and constructed) for full nodes and for light nodes
type FraudService struct {
   badEncodingSub Subscription
   fraudNotifier FraudNotifier
}

func(f *FraudService) Subscribe(ctx context.Context, proofType FraudProofType) (Subscription, error){}

func(f *FraudService) Notify(ctx context.Context, p Proof){}
```
### BEFP verification
Once a light node receives a `BadEncoding` fraud proof, it should:
* verify that merkle proofs corresponds to particular shares(if merkle proof does not correspond to a share, than this BEFP is not valid)
* using `BadEncoding.Shares` light node should re-construct full row or col, compute it's merkle root as in [rsmt2d](https://github.com/celestiaorg/rsmt2d/blob/master/extendeddatacrossword.go#L410) and compare it with merkle root that could be retreived from `dah.DataAvailabilityHeader`(if merkle roots do not match, then this BEFP is not valid)

Light node should stop `DAS`, `Syncer` and `SubmitTx` services, in case if BEFP is valid.
## Status
Proposed

## References

Data Availability(Bad Encoding) Fraud Proofs: [#4](https://github.com/celestiaorg/celestia-node/issues/4)
   
Implement stubs for BadEncodingFraudProofs: [#263](https://github.com/celestiaorg/celestia-node/issues/263) 
