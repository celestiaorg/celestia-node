# ADR #006: Celestia-Node Fraud Service

## Changelog

- 2022.03.03 - init commit
- 2022.03.08 - added pub-sub
- 2022.03.15 - added BEFP verification
- 2022.06.08 -
  - updated rsmt2d error naming(as it was changed in implementation);
  - changed from NamespaceShareWithProof to ShareWithProof;
  - made ProofUnmarshaler public and extended return params;
  - fixed typo issues;
- 2022.06.15 - Extend Proof interface with HeaderHash method;
- 2022.06.22 - Updated rsmt2d to change isRow to Axis;
- 2022.07.03 - Added storage description;
- 2022.07.23 - Reworked unmarshalers registration;
- 2022.08.25 -
  - Added BinaryUnmarshaller to Proof interface;
  - Changed ProofType type from int to string;

## Authors

@vgonkivs @Bidon15 @adlerjohn @Wondertan @renaynay

## Bad Encoding Fraud Proof (BEFP)

## Context

In the case where a Full Node receives `ErrByzantineData` from the [rsmt2d](https://github.com/celestiaorg/rsmt2d) library, it generates a fraud-proof and broadcasts it to DA network such that the light nodes are notified that the corresponding block could be malicious.

## Decision

BEFPs were first addressed in the two issues below:

- <https://github.com/celestiaorg/celestia-node/issues/4>
- <https://github.com/celestiaorg/celestia-node/issues/263>

## Detailed Design

A fraud proof is generated if recovered data does not match with its respective row/column roots during block reparation.

The result of `RepairExtendedDataSquare` will be an error [`ErrByzantineRow`](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L18-L22)/[`ErrByzantineCol`](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L28-L32):

- Both errors consist of
  - row/column numbers that do not match with the Merkle root
  - shares that were successfully repaired and verified (all correct shares).

Based on `ErrByzantineRow`/`ErrByzantineCol` internal fields, we should generate [MerkleProof](https://github.com/celestiaorg/nmt/blob/e381b44f223e9ac570a8d59bbbdbb2d5a5f1ad5f/proof.go#L17) for respective verified shares from [nmt](https://github.com/celestiaorg/nmt) tree return as the `ErrByzantine` from `RetrieveData`.

```go
type ErrByzantine struct {
   // Shares contains all shares from row/col.
   // For non-nil shares MerkleProof is computed
   Shares []*ShareWithProof
   // Index represents the number of row/col where ErrByzantineRow/ErrByzantineColl occurred.
   Index uint8
   // Axis represents the axis that verification failed on.
   Axis rsmt2d.Axis
}

type Share struct {
   Share []byte
   Proof nmt.Proof
}
```

In addition, `das.Daser`:

1. Creates a BEFP:

    ```go
    // Currently, we support only one fraud proof. But this enum will be extended in the future with other
    const (
    BadEncoding ProofType = "badencoding"
    )

    type BadEncodingProof struct {
    Height uint64
    // Shares contains all shares from row/col
    // Shares that did not pass verification in rmst2d will be nil
    // For non-nil shares MerkleProofs are computed
    Shares []*ShareWithProof
    // Index represents the number of row/col where ErrByzantineRow/ErrByzantineColl occurred
    Index uint8
    // Axis represents the axis that verification failed on.
    Axis rsmt2d.Axis
    }
    ```

1. Full node broadcasts BEFP to all light and full nodes via separate sub-service via proto message:

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

    enum axis {
    ROW = 0;
    COL = 1;
    }

    message BadEncoding {
    bytes HeaderHash = 1;
    uint64 Height = 2;
    repeated ipld.pb.Share Shares = 3;
    uint32 Index = 4;
    axis Axis = 5;
    }
    ```

    `das.Daser` imports a data structure that implements `fraud.Broadcaster` interface that uses libp2p.pubsub under the hood:

    ```go
    // Broadcaster is a generic interface that sends a `Proof` to all nodes subscribed on the Broadcaster's topic.
    type Broadcaster interface {
    // Broadcast takes a fraud `Proof` data structure that implements standard BinaryMarshal interface and broadcasts it to all subscribed peers.
    Broadcast(ctx context.Context, p Proof) error
    }
    ```

    ```go
    // ProofType is a enum type that represents a particular type of fraud proof.
    type ProofType string

    // Proof is a generic interface that will be used for all types of fraud proofs in the network.
    type Proof interface {
    Type() ProofType
    HeaderHash() []byte
    Height() (uint64, error)
    Validate(*header.ExtendedHeader) error

    encoding.BinaryMarshaller
    encoding.BinaryUnmarshaler
    }
    ```

    *Note*: Full node, that detected a malicious block and created a Fraud Proof, will also receive it by subscription to stop respective services.

1. From the other side, nodes will, by default, subscribe to the BEFP topic and verify messages received on the topic:

    ```go
    type ProofUnmarshaller func([]byte) (Proof,error)
    // Subscriber encompasses the behavior necessary to
    // subscribe/unsubscribe from new FraudProofs events from the
    // network.
    type Subscriber interface {
    // Subscribe allows to subscribe on pub sub topic by its type.
    // Subscribe should register pub-sub validator on topic.
    Subscribe(ctx context.Context, proofType ProofType) (Subscription, error)
    }
    ```

    ```go
    // Subscription returns a valid proof if one is received on the topic.
    type Subscription interface {
    Proof(context.Context) (Proof, error)
    Cancel() error
    }
    ```

    ```go
    // service implements Subscriber and Broadcaster.
    type service struct {
    pubsub *pubsub.PubSub

    storesLk sync.RWMutex
    stores   map[ProofType]datastore.Datastore

    topics map[ProofType]*pubsub.Topic
   
    getter headerFetcher
    ds     datastore.Datastore
    }

    func(s *service) Subscribe(ctx context.Context, proofType ProofType) (Subscription, error){}
    func(s *service) Broadcast(ctx context.Context, p Proof) error{}
    ```

    BEFP verification

    Once a light node receives a `BadEncodingProof` fraud proof, it should:

    - verify that Merkle proofs correspond to particular shares. If the Merkle proof does not correspond to a share, then the BEFP is not valid.
    - using `BadEncodingProof.Shares`, light node should re-construct full row or column, compute its Merkle root as in [rsmt2d](https://github.com/celestiaorg/rsmt2d/blob/ac0f1e1a51bf7b5420965fb7c35fa32a56e02292/extendeddatacrossword.go#L410) and compare it with Merkle root that could be retrieved from the `DataAvailabilityHeader` inside the `ExtendedHeader`. If Merkle roots match, then the BEFP is not valid.

1. All celestia-nodes should stop some dependent services upon receiving a legitimate BEFP:
Both full and light nodes should stop `DAS`, `Syncer` and `SubmitTx` services.

1. Valid BadEncodingFraudProofs should be stored on the disk using `FraudStore` interface:

### Fraud storage

BEFP storage will be created on first subscription to Bad Encoding Fraud Proof.
BEFP will be stored in datastore once it will be received, using `fraud/badEncodingProof` path and the corresponding block hash as the key:

```go
//  put adds a Fraud Proof to the datastore with the given key.
func put(ctx context.Context, store datastore.Datastore, key datastore.Key, proof []byte) error
```

Once a node starts, it will check if its datastore has a BEFP:

```go
func getAll(ctx context.Context, ds datastore.Datastore) ([][]byte, error)
```

In case if response error will be empty (and not ```datastore.ErrNotFound```), then a BEFP has been already added to storage and the node should be halted.

### Fraud sync

The main purpose of FraudSync is to deliver fraud proofs to nodes that were started after a BEFP appears. Since full nodes create the BEFP during reconstruction, FraudSync is mainly implemented for light nodes:

- Once a light node checks that its local fraud storage is empty, it starts waiting for new connections with the remote peers(full/bridge nodes) using `share/discovery`.
- The light node will send 5 requests to newly connected peers to get a fraud proof.
- If a fraud proof is received from a remote peer, then it should be validated and propagated across all local subscriptions in order to stop the respective services.

NOTE: if a received fraud proof ends up being invalid, then the remote peer will be added to the black list.
Both full/light nodes register a stream handler for handling fraud proof requests.

### Bridge node behaviour

Bridge nodes will behave as light nodes do by subscribing to BEFP fraud sub and listening for BEFPs. If a BEFP is received, it will similarly shut down all dependent services, including broadcasting new `ExtendedHeader`s to the network.

## Status

Proposed

## References

Data Availability(Bad Encoding) Fraud Proofs: [#4](https://github.com/celestiaorg/celestia-node/issues/4)

Implement stubs for BadEncodingFraudProofs: [#263](https://github.com/celestiaorg/celestia-node/issues/263)
