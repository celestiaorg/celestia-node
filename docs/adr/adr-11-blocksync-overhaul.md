# ADR #011: Block Data Sync Overhaul

## Changelog

- 23.08.22: Initial unfinished draft
- 01.09.22: Block/EDS Storage design

## Authors

- @Wondertan

> I start to like writing ADRs, step-by-step. Also, there is a trick that helps: imagine like you are talking to a dev
> who just joined the team to onboard him.

## Glossary

- LN - Light Node
- FN - Full Node
- BN - Bridge Node
- EDS(Extended Data Square) - plain Block data omitting headers and other metadata.

## Context

### Status Qou

Current block data synchronization is done over Bitswap, traversing NMT trees of rows and columns of data square quadrants.
We know from empirical evidence that it takes more than 200 seconds(~65000 network requests) to download a 4MB block of
256kb shares, which is unacceptable and must be much less than the block time(15/30sec).

TODO: Simple diagram with problem visualization

The DASing, on the other hand, shows acceptable metrics for the block sizes we are aiming for initially. In the case of
the same block, a DAS operation takes 50ms * 8(technically 9) blocking requests, which is ~400ms in an ideal scenario
(excluding disk IO). With higher latency and bigger block size(higher NMT trees), the DASIng operation could take much
more(TODO type of the grow, e.g., quadratic/etc.), but is likely(TODO proof?) to be less than a block time.

Getting data by namespace lies between BlockSync and DASing, where more data equals more requests and more time to
fulfill the requests.

### Mini Node Offsite 2022 Berlin

To facilitate and speedup the resolution of the problem we decided to make a team gathering in Berlin for 4 days. With
the help of preliminary preparations by @Wondertan and invited guest @willscott, we were able to find a solution
in 2 days to match the following requirements:

- Sync time less than block time(ideally sub-second)
- Data by namespace less than block time(ideally sub-second)
- Pragmatic timeframe
  - We need this done before incentivized testnet
  - So we don't have time to redesign protocol from scratch
- Keep Bitswap as it suffices DAS and solves data withholding attack
  - Mainly keeping existing Bitswap logic as a fallback mechanism for reconstruction from light nodes case
- Keeping random hash-addressed access to shares for Bitswap to work

### Decision

This ADR is intended to outline design decisions for Block syncing mechanism/protocol improvements together with
block data storage. In a nutshell, the decision is to use ___[CAR format](https://ipld.io/specs/transport/car/carv2/)___
and ___[Dagstore](https://github.com/filecoin-project/dagstore)___ for ___extended block storage___
and ___custom p2p Req/Resp protocol for block data syncing___(whole block and data by namespace id) in the happy path.

#### Key Design Decision

- __FNs/BNs store EDSes as [CAR files](https://ipld.io/specs/transport/car/carv2/).__ CAR format provides an efficient
way to store Merkle DAG data, like EDS with NMT. It packs such DAG data into a single blob which can be read sequentially
in one read and transferred over the wire. Additionally, [CARv2](https://ipld.io/specs/transport/car/carv2/) introduces
pluggable indexes over the blob allowing efficient random access in one read(if the index is cached in memory).

  - __EDSes as _CARv1_ files over _CARv2_.__ CARv2 encodes indexes into the file, however they should not be transferred
in case of EDS, so keeping them separately is a better strategy which `DAGStore` provides out of the box.

- __FNs/BNs run a single instance of `DAGStore` to manage CARv1 block files.__

- __LNs DASing remains untouched__. Both the networking protocol and storage for LNs remains untouched as it fulfills
the requirements. This includes Bitswap as backbone protocol for requesting samples and global Badger KVStore.

- __FNs/BNs manage a top-level index for _hash_ to _CARv1 block file_ mapping.__ Current DASing for LNs requires FNs/BNs to serve
simple hash to data requests. The top-level index maps any hash to any block CARv1file so that FNs/BNs can quickly
serve requests. However, the indexing has a major consequence - data usage, so further, this index will have to be
removed. LNs know which block they sample and can provide this data together with sample request over Bitswap. This
requires us to either facilitate implementation of [Bitswap's auth extention](https://github.com/ipfs/specs/pull/270)
or proposing custom Bitswap message extention.

- __New libp2p based Exchange protocol is introduced for data/share exchange purposes.__ FNs/BNs servers LN client.

### Detailed Design

#### Block/EDS Storage

The new block storage design is solely additive. Meaning that all the existing storage related components and functionality
are kept with additional components introduced. Altogether, existing and new components will be recomposed to serve the
foundation of our improved block EDS storage subsystem.

##### `share.EDSStore`

Mainly, a new addition is `EDSStore` type in `share` pkg, which manages every EDS on the
disk a FNs/BNs keep. Each EDS together with its Merkle Proofs serializes into CARv1 file with a special
traversal algorithm(TODO Explain here or refer as another section). All the serialized CARv1 file blobs are managed in
OS FS via underlying DAGStore.

The introduced `EDSStore` also maintains a top-level index enabling granular and efficient random access to every share
and/or Merkle proof over every registered CARv1 file. The `EDSStore` provides a custom `Blockstore`(TODO link) interface
implementation to achieve the access. However, this comes with additional storage costs for indices. The main use-case
is randomized sampling over the whole chain of EDS block data and getting data by namespace.

```go
type EDSStore struct {
 cars dagstore.DAGStore
 idx index.FullIndexRepo
 mounts  *mount.Registry
 ...
}
```

##### `share.EDSStore.Put`

To write an entire EDS `Put` method is introduced. Internally, it

- Serializes the EDS into the CARv1(*) //TODO Describe in something like EDStoCAR func and its reverse form
- Wraps it with DAGStore's [FileMount](https://github.com/filecoin-project/dagstore/blob/master/mount/file.go#L10)
- Converts `DataRoot` into the `shard.Key`
- Registers the Mount as a shard on the DAGStore

NOTE: Registering on the DAGStore automatically populates top-level index with shares/proofs accessible from stored EDS
so this is out of scope of the document.

```go
// Put stores the given data square with DataRoot as key.
//
// The square is verified on the Exchange level and Put only stores the square trusting it.
// Put serializes the full EDS into a CARv1 file internally and mounts it onto the DAGStore.
// The resulting file stores all the shares and NMT Merkle Proofs of the EDS.
func (s *Store) Put(context.Context, DataRoot, *rsmt2d.ExtendedDataSquare) error
```

##### `share.EDSStore.GetCAR`

To read an EDS as a byte stream `GetCAR` method is introduced. Internally it

- Converts `DataRoot` into the `shard.Key`
- Gets Mount by Key from `mount.Registry`
- Return Reader from `Mount.Fetch`

NOTE: It might be necessary to acquire EDS mount via `DAGStore` keeping `ShardAccessor`
and closing it when operation is done.

```go
// GetCAR takes DataRoot and returns a buffered reader to respective EDS serialized as CARv1 file.
// 
// The Reader strictly reads the first quadrant(1/4) of EDS omitting all the NMT Merkle proofs.
// Integrity of the store data is not verified. 
// 
// Caller must Close returned reader after reading.
func (s *Store) GetCAR(context.Context, DataRoot) (io.ReadCloser, error)
```

##### `share.EDSStore.Blockstore`

`Blockstore` method return a `Blockstore` interface implementation instance, providing random access over share and NMT
Merkle proof in every stored EDS. It is required for FNs/BNs to serve DAS requests over the Bitswap and for reading data
by namespace. // TODO Link to the section

There is a [frozen/un-merged implementation](https://github.com/filecoin-project/dagstore/pull/116) of `Blockstore`
over `DAGStore` and CARv2 indexes.

NOTE: We can either use it(and finish if something is missing for our case) or implement custom optimized for our needs.

```go
// Blockstore returns an IPFS Blockstore providing access to individual shares/nodes of all EDS 
// registered on the Store. NOTE: The Blockstore does not store full Celestia Blocks, but IPFS blocks. 
// We represent `shares` and NMT Merkle proofs as IPFS blocks and IPLD nodes, so that Bitswap can access those.
func (s *Store) Blockstore() blockstore.Blockstore

```

##### `share.EDSStore.Get`

To read an entire EDS `Get` method is introduced. Internally it:

- Gets serialized EDS Reader via `GetCAR`
- Deserializes EDS // TODO Link to its own section
  - Fills new `rsmt2d.ExtendedDataSquare` one by one with [`BlockReader`](https://github.com/ipld/go-car/blob/master/v2/block_reader.go#L16)
- Recomputes and verifies against DataRoot and returns

NOTE: It's not necessary, but API ergonomics/symmetry nice-to-have

```go
// Get reads EDS out of Store by given DataRoot.
// 
// It reads only one quadrant(1/4) of the EDS and verifies integrity of the stored data by recomputing it.
func (s *Store) Get(context.Context, DataRoot) (*rsmt2d.ExtendedDataSquare, error)

```

##### `share.EDSStore.Remove`

To remove stored EDS `Remove` methods is introduced. Internally it:

- Converts `DataRoot` into the `shard.Key`
- Removes Mount/Shard from `DAGStore`
  - `DAGStore` cleans up indices itself
- Removes FS/File mount of CARv1 file from disk

NOTE: It's not necessary, but API ergonomics/symmetry nice-to-have

```go
// Remove removes EDS from Store by given DataRoot.
func (s *Store) Remove(context.Context, DataRoot) error

```

#### Reading Data By Namespace

Generally stays unchanged with minor edits:

- `share/ipld.GetByNamespace` is kept to load data from disk only and not from network anymore
  - Using `Blockservice` with offline exchange // TODO Links
  - Using `Blockstore` provided by `EDSStore`
- `share/ipld.GetByNamespace` is extended to return NMT Merkle proofs
  - Similar to `share/ipld.GetProofsForShares`
  - Ensure merkle proofs are not duplicated!

Alternatively, `share/ipld.GetByNamespace` can be modified to `share.CARByNamespace` returning
CARv1 Reader with encoded shares and NMT Merkle Proofs.

##### `node.Store`

// TODO Elaborate on how FS/File DAGStore mounts work over `node.Store` + `index.FullIndexRepo`

##### Relations with Node Types

// TODO Elaborate on node relations

### Alternative Approaches

#### Block Storage

- Extended blocks as a set of share blobs and Merkle proofs in global store (_current approach with KVStore_)
- Extended block as a single blob only(computing Merkle proofs)
- Extended block as a single blob and Merkle proofs
- Extended block as a set of DAG/CAR blobs
- Extended block as a single DAG/CAR blob

#### Block Syncing

- GraphSync
- Bitswap(current)

### Considerations

- EDS to/from CARv2 converting performance
Current sync design assumes two converts from CAR to EDS on the protocol layer and back to CAR when storing the EDS.
Rsmt2d allocates on most operations with individual shares and for bigger blocks during sync this allocs puts significant
pressure on GC. One way to substantially alleviate this is to integrate bytes buffer pool into rmst2d
