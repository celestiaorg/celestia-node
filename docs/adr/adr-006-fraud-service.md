# ADR #006: Celestia-Node Fraud Service

## Changelog

- 2022.03.03 - init commit
- 2022.03.08 - added pub-sub
- 2022.03.15 - added BEFP verification

## Authors

@vgonkivs @Bidon15 @adlerjohn @Wondertan @renaynay
 
## Bad Encoding Fraud Proof (BEFP)
## Context

In the case where a Full Node receives `ErrByzantineRow`/`ErrByzantineCol` from the [rsmt2d](https://github.com/celestiaorg/rsmt2d) library, it generates a fraud-proof and broadcasts it to DA network such that the light nodes are notified that the corresponding block could be malicious.

## Decision

BEFPs were first addressed in the two issues below:

- https://github.com/celestiaorg/celestia-node/issues/4
- https://github.com/celestiaorg/celestia-node/issues/263

## Detailed Design
A fraud proof is generated if recovered data does not match with its respective row/column roots during block reparation. 

The result of `RepairExtendedDataSquare` will be an error [`ErrByzantineRow`](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L18-L22)/[`ErrByzantineCol`](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L28-L32):

- Both errors consist of 
  - row/column numbers that do not match with the Merkle root
  - shares that were successfully repaired and verified (all correct shares).

Based on `ErrByzantineRow`/`ErrByzantineCol` internal fields, we should generate [MerkleProof](https://github.com/celestiaorg/nmt/blob/e381b44f223e9ac570a8d59bbbdbb2d5a5f1ad5f/proof.go#L17) for respective verified shares from [nmt](https://github.com/celestiaorg/nmt) tree return as the `ErrBadEncoding` from `RetrieveData`. 

```go
type ErrBadEncoding struct {
   // Shares contains all shares from row/col.
   // For non-nil shares MerkleProof is computed
   Shares []*Share
   // Position represents the number of row/col where ErrByzantineRow/ErrByzantineColl occurred.
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
   BadEncoding ProofType = 0
)

type BadEncodingProof struct {
   Height uint64
   // Shares contains all shares from row/col
   // Shares that did not pass verification in rmst2d will be nil
   // For non-nil shares MerkleProofs are computed
   Shares []*Share
   // Position represents the number of row/col where ErrByzantineRow/ErrByzantineColl occurred
   Position uint8
   isRow bool
}
```

2. Full node broadcasts BEFP to all light nodes via separate sub-service via proto message:

```proto3

message MerkleProof {
   int64          start     = 1;
   int64          end       = 2;
   repeated bytes nodes     = 3;
   bytes leaf_hash          = 4;
}

message ShareWithProof {
   bytes Share = 1;
   MerkleProof Proof = 2;
}

message BadEnconding {
   required uint64 Height = 1;
   repeated Share Shares = 2;
   uint8 Index = 3;
   bool isRow = 4;
}
```

`das.Daser` imports a data structure that implements `proof.Broadcaster` interface that uses libp2p.pubsub under the hood:

```go
// Broadcaster is a generic interface that sends a `Proof` to all nodes subscribed on the Broadcaster's topic.
type Broadcaster interface {
   // Broadcast takes a fraud `Proof` data structure that implements standard BinaryMarshal interface and broadcasts it to all subscribed peers.
   Broadcast(ctx context.Context, p Proof) error
}
```

```go
// ProofType is a enum type that represents a particular type of fraud proof.
type ProofType int

// Proof is a generic interface that will be used for all types of fraud proofs in the network.
type Proof interface {
   Type() ProofType
   Height() (uint64, error)
   Validate(*header.ExtendedHeader) error

   encoding.BinaryMarshaller
}
```

2a. From the other side, light nodes will, by default, subscribe to the BEFP topic and verify messages received on the topic:

```go
type proofUnmarshaller func([]byte) (Proof error)
// Subscriber encompasses the behavior necessary to
// subscribe/unsubscribe from new FraudProofs events from the
// network.
type Subscriber interface {
   // Subscribe allows to subscribe on pub sub topic by it's type.
   // Subscribe should register pub-sub validator on topic.
   Subscribe(ctx context.Context, proofType ProofType) (Subscription, error)
   // RegisterUnmarshaller registers unmarshaller for the given ProofType.
   // If there is no umarshaller for `ProofType`, then `Subscribe` returns an error.
   RegisterUnmarshaller(proofType ProofType, f proofUnmarshaller) error
   // UnregisterUnmarshaller removes unmarshaller for the given ProofType.
   // If there is no unmarshaller for `ProofType`, then it returns an error.
   UnregisterUnmarshaller(proofType ProofType) error{}
}
```

```go
// Subscription returns a valid proof if one is received on the topic.
type Subscription interface {
   Proof(context.Context) (Proof, error)
   Cancel()
}
```

```go
// Fraudsub implements Subscriber and Broadcaster.
type Fraudsub struct {
   pubsub *pubsub.PubSub
   topics map[ProofType]*pubsub.Topic
   unmarshallers map[ProofType]proofUnmarshaller
}

func(s *Fraudsub) RegisterUnmarshaller(proofType ProofType, f proofUnmarshaller) error{}
func(s *Fraudsub) UnregisterUnmarshaller(proofType ProofType) error{}

func(s *Fraudsub) Subscribe(ctx context.Context, proofType ProofType) (Subscription, error){}
func(s *Fraudsub) Broadcast(ctx context.Context, p Proof) error{}
```
### BEFP verification
Once a light node receives a `BadEncodingProof` fraud proof, it should:
* verify that Merkle proofs correspond to particular shares. If the Merkle proof does not correspond to a share, then the BEFP is not valid.
* using `BadEncodingProof.Shares`, light node should re-construct full row or column, compute its Merkle root as in [rsmt2d](https://github.com/celestiaorg/rsmt2d/blob/ac0f1e1a51bf7b5420965fb7c35fa32a56e02292/extendeddatacrossword.go#L410) and compare it with Merkle root that could be retrieved from the `DataAvailabilityHeader` inside the `ExtendedHeader`. If Merkle roots match, then the BEFP is not valid.

3. All celestia-nodes should stop some dependent services upon receiving a legitimate BEFP:
Both full and light nodes should stop `DAS`, `Syncer` and `SubmitTx` services.

4. Valid BadEncodingFraudProofs should be stored on the disk using `FraudStore` interface:

```go
type FraudStore interface {
   // Put stores given Proof by header's hash in respective for this fraud proof directory
   Put(path FraudProofType, headerHash string, p Proof) error
   // Get retrieves Proof by header's hash from respective for this fraud proof directory
   Get(path FraudProofType, headerHash string) (Proof, error)
   // GetMany retrieves all Proofs from the respective for this fraud proof directory
   GetMany(path FraudProofType) ([]Proof, error)
}

type fraudstore struct{
   path string
}
```
### Bridge node behaviour
Bridge nodes will behave as light nodes do by subscribing to BEFP fraud sub and listening for BEFPs. If a BEFP is received, it will similarly shut down all dependent services, including broadcasting new `ExtendedHeader`s to the network.

## Status
Proposed

## References

Data Availability(Bad Encoding) Fraud Proofs: [#4](https://github.com/celestiaorg/celestia-node/issues/4)
   
Implement stubs for BadEncodingFraudProofs: [#263](https://github.com/celestiaorg/celestia-node/issues/263) 
