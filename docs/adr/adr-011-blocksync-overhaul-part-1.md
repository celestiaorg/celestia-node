# ADR #011: Block Data Sync Overhaul: Part I - Storage

## Changelog

- 23.08.22: Initial unfinished draft
- 14.09.22: The first finished version
- 02.12.22: Fixing missed gaps

## Authors

- @Wondertan

> I start to like writing ADRs step-by-step. Also, there is a trick that helps: imagine like you are talking to a dev
> who just joined the team to onboard him.

## Glossary

- LN - Light Node
- FN - Full Node
- BN - Bridge Node
- [EDS(Extended Data Square)][eds] - plain Block data omitting headers and other block metadata
- ODS - Original Data Square or the first quadrant of the EDS. Contains real user data and padding
- [NMT][nmt] - Namespaced Merkle Tree
- [DataHash][dh] - Hash commitment over [DAHeader][dah]

## Context

### Status Quo

Current block data synchronization is done over Bitswap, traversing NMT trees of rows and columns of data square quadrants.
We know from empirical evidence that it takes more than 200 seconds(~65000 network requests) to download a 4MB block of
256kb shares, which is unacceptable and must be much less than the block time(15-30sec).

The DASing, on the other hand, shows acceptable metrics for the block sizes we are aiming for initially. In the case of
the same block, a DAS operation takes 50ms * 8(technically 9) blocking requests, which is ~400ms in an ideal scenario
(excluding disk IO).

Getting data by namespace also needs to be improved. The time it takes currently lies between BlockSync and DASing,
where more data equals more requests and more time to fulfill the requests.

### Mini Node Offsite 2022 Berlin

To facilitate and speed up the resolution of the problem, we decided to make the team gathering in Berlin for four days.
With the help of preliminary preparations by @Wondertan and guest @willscott, we were able to find a solution
in 2 days to match the following requirements:

- Sync time less than block time(ideally sub-second)
- Data by namespace less than block time(ideally sub-second)
- Pragmatic timeframe
  - We need this done before incentivized testnet
  - We don't have time to redesign the protocol from scratch
- Keep Bitswap, as it suffices for DAS and solves the data withholding attack
  - Existing Bitswap logic kept as a fallback mechanism for the case of reconstruction from light nodes
- Keeping random hash-addressed access to shares for Bitswap to work

## Decision

This ADR intends to outline design decisions for block data storage. In a nutshell, the decision is to use
___[CAR format][car]___ and ___[Dagstore][dagstore]___
for ___extended block storage___ and ___custom p2p Req/Resp protocol for block data syncing___(whole block and data by
namespace id) in the happy path. The p2p portion of the document will come in the subsequent Part II document.

### Key Design Decisions

- __FNs/BNs store EDSes serialized as [CAR files][car].__ CAR format provides an
  efficient way to store Merkle DAG data, like EDS with NMT. It packs such DAG data into a single blob which can be read
  sequentially in one read and transferred over the wire. Additionally, [CARv2][carv2]
  introduces pluggable indexes over the blob allowing efficient random access to shares and NMT Proofs in one read
  (if the index is cached in memory).

- __FNs/BNs manage a top-level index for _hash_ to _CAR block file_ mapping.__ Current DASing for LNs requires FNs/BNs
  to serve simple hash to data requests. The top-level index maps any Share/NMT Proof hash to any block CARv1 file so
  that FNs/BNs can quickly serve DA requests.

- __FNs/BNs address EDSes by `DataHash`.__ The only alternative is by height; however, it does not allow block
  data deduplication in case EDSes are equal and couples the Data layer/pkg with the Header layer/pkg.

- __FNs/BNs run a single instance of [`DAGStore`][dagstore] to manage CAR block
  files.__ DAGStore provides the top-level indexing and CARv2-based indexing per each CAR file. In essence, it's an
  engine for managing any CAR files with indexing, convenient abstractions, tools, recovery mechanisms, etc.
  - __EDSes as _CARv1_ files over _CARv2_.__ CARv2 encodes indexes into the file, while DAGStore maintains CARv2-based
      indexing. Usage of CARv1 keeps only one copy of the index, stores/transfers less metadata per EDS, and simplifies
      reading EDS from a file.

- __LNs DASing remains untouched__. The networking protocol and storage for LNs remain intact as it fulfills the
  requirements. Bitswap usage as the backbone protocol for requesting samples and global Badger KVStore remain unaffected.

### Detailed Design

> All the comments on the API definitions should be preserved and potentially improved by implementations.

The new block storage design is solely additive. All the existing storage-related components and functionality
are kept with additional components introduced. Altogether, existing and new components will be recomposed to serve as the
foundation of our improved block storage subsystem.

The central data structure representing Celestia block data is EDS(`rsmt2d.ExtendedDataSquare`), and the new storage design
is focused around storing entire EDSes as a whole rather than a set of individual shares, s.t. storage subsystem
can handle storing and streaming/serving blocks of 4MB and more.

#### EDS (De-)Serialization

Storing EDS as a whole requires EDS (de)serialization. For this, the [CAR format][car] is chosen.

##### `eds.WriteEDS`

To write EDS into a stream/file, `WriteEDS` is introduced. Internally, it

- [Re-imports](https://github.com/celestiaorg/rsmt2d/blob/80d231f733e9dd8ca166c3d670470ed9a1c165d9/extendeddatasquare.go#L44) EDS similarly to
  [`ipld.ImportShares`](https://github.com/celestiaorg/celestia-node/blob/da4f54bca1bfef86f53880ced569d37ffb4b8b84/share/add.go#L48)
  - Using [`Blockservice`][blockservice] with [offline
      exchange][offexchange] and in-memory [`Blockstore`][blockstore]
  - With [`NodeVisitor`](https://github.com/celestiaorg/celestia-node/blob/da4f54bca1bfef86f53880ced569d37ffb4b8b84/share/add.go#L63), which saves to the
      [`Blockstore`][blockstore] only NMT Merkle proofs(no shares) _NOTE: `len(node.Links()) == 2`_
    - Actual shares are written further in a particular way explained further
- Creates and [writes](https://github.com/ipld/go-car/blob/dab0fd5bb19dead0da1377270f37be9acf858cf0/car.go#L86) header [`CARv1Header`](https://github.com/ipld/go-car/blob/dab0fd5bb19dead0da1377270f37be9acf858cf0/car.go#L30)
  - Fills up `Roots` field with `EDS.RowRoots/EDS.ColRoots` roots converted into CIDs
- Iterates over shares in quadrant-by-quadrant order via `EDS.GetCell`
  - [Writes](https://github.com/ipld/go-car/blob/dab0fd5bb19dead0da1377270f37be9acf858cf0/car.go#L118) the shares in row-by-row order
- Iterates over in-memory Blockstore and [writes](https://github.com/ipld/go-car/blob/dab0fd5bb19dead0da1377270f37be9acf858cf0/car.go#L118) NMT Merkle
  proofs stored in it

___NOTES:___

- _CAR provides [a utility](https://github.com/ipld/go-car/blob/dab0fd5bb19dead0da1377270f37be9acf858cf0/car.go#L47) to serialize any DAG into the file
  and there is a way to serialize EDS into DAG(`share/ipld.ImportShares`). This approach is the simplest and traverses
  shares and Merkle Proofs in a depth-first manner packing them in a CAR file. However, this is incompatible with the
  requirement of being able to truncate the CAR file reading out __only__ the first quadrant out of it without NMT proofs,
  so serialization must be different from the utility to support that._
- _Alternatively to `WriteEDS`, an `EDSReader` could be introduced to make EDS-to-stream handling more idiomatic
  and efficient in some cases, with the cost of more complex implementation._

```go
// WriteEDS writes the whole EDS into the given io.Writer as CARv1 file.
// All its shares and recomputed NMT proofs.
func WriteEDS(context.Context, *rsmt2d.ExtendedDataSquare, io.Writer) error
```

##### `eds.ReadEDS`

To read an EDS out of a stream/file, `ReadEDS` is introduced. Internally, it

- Imports EDS with an empty pre-allocated slice. _NOTE: Size can be taken from CARHeader.
- Wraps given `io.Reader` with [`BlockReader`](https://github.com/ipld/go-car/blob/dab0fd5bb19dead0da1377270f37be9acf858cf0/v2/block_reader.go#L17)
- Reads out blocks one by one and fills up the EDS quadrant via `EDS.SetCell`
  - In total there should be shares-in-quadrant amount of reads.
- Recomputes and validates via `EDS.Repair`

```go
// ReadEDS reads an EDS quadrant(1/4) from an io.Reader CAR file.
// It expects strictly first EDS quadrant(top left).
// The returned EDS is guaranteed to be full and valid against the DataHash, otherwise ReadEDS errors.
func ReadEDS(context.Context, io.Reader, DataHash) (*rsmt2d.ExtendedDataSquare, error)
```

##### `eds.ODSReader`

To read only a quadrant/ODS out of full EDS, `ODSReader` is introduced.

Its constructor wraps any `io.Reader` containing EDS generated by `WriteEDS` and produces an `io.Reader` which reads
exactly an ODS out of it, similar to `io.LimitReader`. The size of the EDS and ODS can be determined by the amount
of CIDs in the `CARHeader`.

#### `eds.Store`

FNs/BNs keep an `eds.Store` to manage every EDS on the disk. The `eds.Store` type is introduced in the `eds` pkg.
Each EDS together with its Merkle Proofs serializes into a CARv1 file. All the serialized CARv1 file blobs are mounted on
DAGStore via [Local FS Mounts](https://github.com/filecoin-project/dagstore/blob/master/docs/design.md#mounts) and registered
as [Shards](https://github.com/filecoin-project/dagstore/blob/master/docs/design.md#shards).

The introduced `eds.Store` also maintains (via DAGStore) a top-level index enabling granular and efficient random access
to every share and/or Merkle proof over every registered CARv1 file. The `eds.Store` provides a custom `Blockstore` interface
implementation to achieve access. The main use-case is randomized sampling over the whole chain of EDS block data
and getting data by namespace.

```go
type Store struct {
 basepath string
 dgstr dagstore.DAGStore
 topIdx index.Inverted
 carIdx index.FullIndexRepo
 mounts  *mount.Registry
 ...
}

// NewStore constructs Store over OS directory to store CARv1 files of EDSes and indices for them.
// Datastore is used to keep the inverted/top-level index.
func NewStore(basepath string, ds datastore.Batching) *Store {
 topIdx := index.NewInverted(datastore)
 carIdx := index.NewFSRepo(basepath + "/index")
 mounts := mount.NewRegistry()
 r := mount.NewRegistry()
 err = r.Register("fs", &mount.FSMount{FS: os.DirFS(basePath + "/eds/")}) // registration is a must
 if err != nil {
  panic(err)
 }

 return &Store{
  basepath: basepath,
  dgst: dagstore.New(dagstore.Config{...}),
  topIdx: index.NewInverted(datastore),
  carIdx: index.NewFSRepo(basepath + "/index")
  mounts: mounts,
 }
}
```

___NOTE:___ _EDSStore should have lifecycle methods(Start/Stop)._

##### `eds.Store.Put`

To write an entire EDS `Put` method is introduced. Internally it

- Opens a file under `Store.Path/DataHash` path
- Serializes the EDS into the file via `share.WriteEDS`
- Wraps it with `DAGStore`'s [FileMount][filemount]
- Converts `DataHash` to the [`shard.Key`][shardkey]
- Registers the `Mount` as a `Shard` on the `DAGStore`

___NOTE:___ _Registering on the DAGStore populates the top-level index with shares/proofs accessible from stored EDS, which is
out of the scope of the document._

```go
// Put stores the given data square with DataHash as a key.
//
// The square is verified on the Exchange level and Put only stores the square trusting it.
// The resulting file stores all the shares and NMT Merkle Proofs of the EDS.
// Additionally, the file gets indexed s.t. Store.Blockstore can access them.
func (s *Store) Put(context.Context, DataHash, *rsmt2d.ExtendedDataSquare) error
```

##### `eds.Store.GetCAR`

To read an EDS as a byte stream `GetCAR` method is introduced. Internally it

- Converts `DataHash` to the [`shard.Key`][shardkey]
- Acquires `ShardAccessor` and returns `io.ReadCloser` from it

___NOTES:___

- _`DAGStores`'s `ShardAccessor` has to be extended to return an `io.ReadCloser`. Currently, it only returns
  a `Blockstore` of the CAR_
- _The returned `io.ReadCloer` represents full EDS exchanged. To get a quadrant an ODSReader should be used instead_

```go
// GetCAR takes a DataHash and returns a buffered reader to the respective EDS serialized as a CARv1 file.
//
// The Reader strictly reads our full EDS, and it's integrity is not verified.
//
// Caller must Close returned reader after reading.
func (s *Store) GetCAR(context.Context, DataHash) (io.ReadCloser, error)
```

##### `eds.Store.Blockstore`

`Blockstore` method returns a [`Blockstore`][blockstore] interface implementation instance, providing random access over
share and NMT Merkle proof in every stored EDS. It is required for FNs/BNs to serve DAS requests over the Bitswap.

There is a `Blockstore` over [`DAGStore`][dagstore] and [`CARv2`][carv2] indexes.

___NOTES:___

- _We can either use DAGStore's one or implement custom optimized for our needs._
- _The Blockstore does not store whole Celestia Blocks, but IPFS blocks. We represent Merkle proofs and shares in IPFS
  blocks._
- EDIT: We went with custom implementation.

```go
// Blockstore returns an IPFS Blockstore providing access to individual shares/nodes of all EDS
// registered on the Store. NOTE: The Blockstore does not store whole Celestia Blocks but IPFS blocks.
// We represent `shares` and NMT Merkle proofs as IPFS blocks and IPLD nodes so Bitswap can access those.
func (s *Store) Blockstore() blockstore.Blockstore
```

##### `eds.Store.CARBlockstore`

`CARBlockstore` method returns a read-only [`Blockstore`][blockstore] interface implementation
instance to provide random access over share and NMT Merkle proof in a specific EDS identified by
DataHash, along with its corresponding DAH. It is required for FNs/BNs to enable [reading data by
namespace](#reading-data-by-namespace).

___NOTES:___

- _The returned Blockstore does not store whole Celestia Blocks, but IPFS blocks. We represent Merkle proofs and shares in IPFS
  blocks._

```go
// CARBlockstore returns an IPFS Blockstore providing access to individual shares/nodes of a specific EDS identified by
// DataHash and registered on the Store. NOTE: The Blockstore does not store whole Celestia Blocks but IPFS blocks.
// We represent `shares` and NMT Merkle proofs as IPFS blocks and IPLD nodes so Bitswap can access those.
func (s *Store) CARBlockstore(context.Context, DataHash)  (dagstore.ReadBlockstore, error)
```

##### `eds.Store.GetDAH`

The `GetDAH` method returns the DAH (`share.Root`) of the EDS identified by `DataHash`. Internally it:

- Acquires a `ShardAccessor` for the corresponding shard
- Reads the CAR Header from the accessor
- Converts the header's root CIDs into a `share.Root`
- Verifies the integrity of the `share.Root` by comparing it with the `DataHash`

```go
// GetDAH returns the DataAvailabilityHeader for the EDS identified by DataHash.
func (s *Store) GetDAH(context.Context, share.DataHash) (*share.Root, error)
```

##### `eds.Store.Get`

To read an entire EDS `Get` method is introduced. Internally it:

- Gets a serialized EDS `io.Reader` via `Store.GetCAR`
- Deserializes the EDS and validates it via `share.ReadEDS`

___NOTE:___ _It's unnecessary, but an API ergonomics/symmetry nice-to-have._

```go
// Get reads EDS out of Store by given DataHash.
//
// It reads only one quadrant(1/4) of the EDS and verifies the integrity of the stored data by recomputing it.
func (s *Store) Get(context.Context, DataHash) (*rsmt2d.ExtendedDataSquare, error)
```

##### `eds.Store.Has`

To check if EDSStore keeps an EDS `Has` method is introduced. Internally it:

- Converts `DataHash` to the [`shard.Key`][shardkey]
- Checks if [`GetShardInfo`](https://github.com/filecoin-project/dagstore/blob/f9e7b7b4594221c8a4840a1e9f3f6e003c1b4c52/dagstore.go#L483) does not return
  [ErrShardUnknown](https://github.com/filecoin-project/dagstore/blob/eac7733212fdd7c80be5078659f7450b3956d2a6/dagstore.go#L55)

___NOTE:___ _It's unnecessary, but an API ergonomics/symmetry nice-to-have._

```go
// Has checks if EDS exists by the given DataHash.
func (s *Store) Has(context.Context, DataHash) (bool, error)
```

##### `eds.Store.Remove`

To remove stored EDS `Remove` method is introduced. Internally it:

- Converts `DataHash` to the [`shard.Key`][shardkey]
- Destroys `Shard` via `DAGStore`
  - Internally removes its `Mount` as well
- Removes CARv1 file from disk under `Store.Path/DataHash` path
- Drops indecies

___NOTES:___

- _It's unnecessary, but an API ergonomics/symmetry nice-to-have_
- _GC logic on the DAGStore has to be investigated so that Removing is correctly implemented_

```go
// Remove removes EDS from Store by the given DataHash and cleans up all the indexing.
func (s *Store) Remove(context.Context, DataHash) error
```

#### Reading Data By Namespace

Generally stays unchanged with minor edits:

- `share/ipld.GetByNamespace` is kept to load data from disk only and not from the network anymore
  - Using [`Blockservice`][blockservice] with [offline exchange][offexchange]
  - Using [`Blockstore`][blockstore] provided by `eds.Store`
- `share/ipld.GetByNamespace` is extended to return NMT Merkle proofs
  - Similar to `share/ipld.GetProofsForShares`
  - Ensure Merkle proofs are not duplicated!

As an extension, `share/ipld.GetByNamespace` can be modified to `share.CARByNamespace`, returning CARv1 Reader with
encoded shares and NMT Merkle Proofs.

#### EDS Deduplication

Addressing EDS by DataHash allows us to deduplicate equal EDSes. EDS equality is very unlikely to happen in practice,
beside empty Block case, which always produces the same EDS.

#### Empty Block/EDS

The empty block is valid and small EDS. It can happen in the early stage of the network. Its body is constant, and to avoid
transferring it over the wire, the `eds.Store` should be pre-initialized with an empty EDS value.

#### EDSStore Directory Path

The EDSStore on construction expects a directory to store CAR files and indices. The path should be gotten based
on `node.Store.Path`.

## Alternative Approaches

- Extended blocks as a set of share blobs and Merkle proofs in global Store (_current approach with KVStore_)
- Extended block as a single blob only(computing Merkle proofs)
- Extended block as a single blob and Merkle proofs
- Extended block as a set of DAG/CAR blobs
- Extended block as a single DAG/CAR blob

## Considerations

- ___EDS to/from CARv2 converting performance.___ Current sync design assumes two converts from CAR to EDS on the
  protocol layer and back to CAR when storing the EDS. Rsmt2d allocates on most operations with individual shares, and for
  more giant blocks during sync, these allocations put significant pressure on GC. One way to substantially alleviate this
  is to integrate the bytes buffer pool into rmst2d.

- ___Disk usage increases from the top-level index.___ This is a temporary solution. The index will have to be removed.
  LNs know which block they sample and can provide `DataHash`together with sample request over Bitswap, removing
  the need for hash-to-eds-file mapping. This requires us to either facilitate implementation of [Bitswap's auth extension
  ](https://github.com/ipfs/specs/pull/270) or proposing a custom Bitswap message extension. Subsequently, the Blockstore
  implementation provided via `eds.Store` would have to be changed to expect DataHash to be passed through the
  `context.Context`.

[dah]: https://github.com/celestiaorg/celestia-app/blob/86c9bf6b981a8b25033357fddc89ef70abf80681/pkg/da/data_availability_header.go#L28
[dh]: https://github.com/celestiaorg/celestia-core/blob/f76d026f3525d2d4fa309c62df29d42d33d0e9c6/types/block.go#L354
[eds]: https://github.com/celestiaorg/rsmt2d/blob/76b270f80f0b9ac966c6f6b043e31514574f90f3/extendeddatasquare.go#L10
[nmt]: https://github.com/celestiaorg/nmt
[car]: https://ipld.io/specs/transport/car
[carv2]: https://ipld.io/specs/transport/car/carv2/
[dagstore]: https://github.com/filecoin-project/dagstore
[blockstore]: https://github.com/ipfs/go-ipfs-blockstore/blob/master/blockstore.go#L33
[blockservice]: https://github.com/ipfs/go-blockservice/blob/master/blockservice.go#L46
[offexchange]: https://github.com/ipfs/go-ipfs-exchange-offline/blob/master/offline.go#L16
[shardkey]: https://github.com/filecoin-project/dagstore/blob/f9e7b7b4594221c8a4840a1e9f3f6e003c1b4c52/shard/key.go#L12
[filemount]: https://github.com/filecoin-project/dagstore/blob/f9e7b7b4594221c8a4840a1e9f3f6e003c1b4c52/mount/file.go#L10
