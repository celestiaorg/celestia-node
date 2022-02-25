# CHANGELOG
## v0.2.0 | 2022-02-23
The second minor release focuses on the stability and robustness of existing features via refactorings, mainly for
headers synchronization and storing, while fixing the DA network segregation by providing bootstrap peers, allowing you to
run a Light Node with zero effort. The release also includes a few breaking changes.

### Highlights
#### Hardcoded bootstrap peers
Bootstrappers are essential backbone peers that every other peer is connected to be part of the DA p2p network.
Additionally, they serve block headers and shares for everyone, so any other node joining the network can use them to
synchronize headers and perform Data Availability Sampling. Now it's unnecessary to maintain your own Bridge(previously
Full) Node to run a Light Node, as now it will rely on bootstrappers by default.

#### Zero effort Light Node
Now becoming a valuable node that contributes to the security of the of Celestia network is only two commands away. Simply run `celestia light init` followed by `celestia
light start`, and you are good to go.

#### Test Swamp
The release comes with a new internal testing library for Celestia Node called Test Swamp, aimed to provide an
ergonomic framework for integration tests for `celestia-node`. It allows simulating a network on which we can test various
high-level scenarios.

#### Header Synchronization
It was almost rewritten from scratch with a better design which:
* Tolerates network disconnections
* Tolerates primary network interface changes
  * both above are useful for Light Node running on laptops or mobile phones
* Optimizes bandwidth and IO usage, subsequently speeding up the synchronization time for ~20%
* Eliminates long-standing issue with header duplicates flooding the network
* Fixes the most common issue community has faced - constant logging of __invalid headers__ error.

#### Renamed Full Node
Full Node is now called Bridge Node to emphasize its purpose of bridging the core consensus and the halo DA networks, both
powering the Celestia project. 

> Spoiler: Next release will come with a reincarnation of the Full Node type, which operates only over the DA `celestia-node` network

Full node operators after the update should now migrate to Bridge Node. This migration is trivial:
* Rename `.celestia-full` to `.celestia-bridge`
* Change scripts from `celestia full` to `celestia bridge`

#### Trusted Peers
* `trusted-peer` is now `trusted-peers` as the flag now allows passing multiple trusted peers.
* `Config.Services.TrustedPeer` is now `Config.Services.TrustedPeers`

### BREAKING CHANGES

- [node: Light node can be initialised with multiple trusted peers #455](https://github.com/celestiaorg/celestia-node/pull/455) [@vgonkivs](https://github.com/vgonkivs)
- [chore: rename Repository to Store #296](https://github.com/celestiaorg/celestia-node/pull/296) [@Wondertan](https://github.com/Wondertan)
- [chore: rename Full node to Bridge node #294](https://github.com/celestiaorg/celestia-node/pull/294) [@Wondertan](https://github.com/Wondertan)
- [node: remove InitWith #291](https://github.com/celestiaorg/celestia-node/pull/291) [@Wondertan](https://github.com/Wondertan)

### FEATURES

- [feat(cmd/cel-shed): new header category and store-init cmd #462](https://github.com/celestiaorg/celestia-node/pull/462) [@Wondertan](https://github.com/Wondertan)
- [feat(cmd): cli flag to enable http/pprof handler to capture profiles #463](https://github.com/celestiaorg/celestia-node/pull/463) [@Wondertan](https://github.com/Wondertan)
- [service/header: SyncState #397](https://github.com/celestiaorg/celestia-node/pull/397) [@Wondertan](https://github.com/Wondertan)
- [feat(service/header): HeightSub #428](https://github.com/celestiaorg/celestia-node/pull/428) [@Wondertan](https://github.com/Wondertan)
- [params: Define Network Types #346](https://github.com/celestiaorg/celestia-node/pull/346) [@Wondertan](https://github.com/Wondertan)
- [feat(cmd): give a birth to cel-shed and p2p key utilities #281](https://github.com/celestiaorg/celestia-node/pull/281) [@Wondertan](https://github.com/Wondertan)
- [feat(cmd|node): MutualPeers Node option and CLI flag #280](https://github.com/celestiaorg/celestia-node/pull/280) [@Wondertan](https://github.com/Wondertan)
- [node: enhance DI allowing overriding of dependencies](https://github.com/celestiaorg/celestia-node/pull/290) [@Wondertan](https://github.com/Wondertan)
- [ci: create docker build GH action](https://github.com/celestiaorg/celestia-node/pull/338) [@jbowen93](https://github.com/jbowen93)
- [swamp: initial structure of the tool](https://github.com/celestiaorg/celestia-node/pull/315) [@Bidon15](https://github.com/Bidon15)

### IMPROVEMENTS
- [feat(node): add go-watchdog to curb OOMs #466](https://github.com/celestiaorg/celestia-node/pull/466) [@Wondertan](https://github.com/Wondertan)
- [perf(node/store): fine-tune Badgerdb params #465](https://github.com/celestiaorg/celestia-node/pull/465) [@Wondertan](https://github.com/Wondertan)
- [feat(service/header): update Store.Append to return amount of applied/valid headers #434](https://github.com/celestiaorg/celestia-node/pull/434) [@Wondertan](https://github.com/Wondertan)
- [refactor(service/header): rework on disk writing strategy of the Store #431](https://github.com/celestiaorg/celestia-node/pull/431) [@Wondertan](https://github.com/Wondertan)
- [refactor(service/header): extract store initialization from Syncer #430](https://github.com/celestiaorg/celestia-node/pull/430) [@Wondertan](https://github.com/Wondertan)
- [header: hardening syncing logic #334](https://github.com/celestiaorg/celestia-node/pull/334) [@Wondertan](https://github.com/Wondertan)
- [feat(params): add bootstrappers #399](https://github.com/celestiaorg/celestia-node/pull/399) [@Wondertan](https://github.com/Wondertan)
- [service/header: remove start/stop from P2PExchange](https://github.com/celestiaorg/celestia-node/pull/367) [@Bidon15](https://github.com/Bidon15)
- [service/share: Implement `FullAvailability`](https://github.com/celestiaorg/celestia-node/pull/333) [@renaynay](https://github.com/renaynay)
- [services/header: Refactor `HeaderService` to be responsible for broadcasting new `ExtendedHeader`s to the gossipsub network](https://github.com/celestiaorg/celestia-node/pull/327) [@renaynay](https://github.com/renaynay)
- [cmd: introduce Env - an Environment for CLI commands #313](https://github.com/celestiaorg/celestia-node/pull/313) [@Wondertan](https://github.com/Wondertan)
- [node: Adding WithHost options to settings section #301](https://github.com/celestiaorg/celestia-node/pull/301) [@Bidon15](https://github.com/Bidon15)
- [node: Adding WithCoreClient option #305](https://github.com/celestiaorg/celestia-node/pull/305) [@Bidon15](https://github.com/Bidon15)
- [service/header: Refactor `HeaderService` to only manage its sub-services' lifecycles #317](https://github.com/celestiaorg/celestia-node/pull/317) [@renaynay](https://github.com/renaynay)
- [docker: Created `docker/` dir with `Dockerfile` and `entrypoint.sh` script](https://github.com/celestiaorg/celestia-node/pull/295) [@jbowen93](https://github.com/jbowen93)
- [chore(share): handle rows concurrently in GetSharesByNamespace #241](https://github.com/celestiaorg/celestia-node/pull/241) [@vgonkivs](https://github.com/vgonkivs)
- [ci: adding data race detector action](https://github.com/celestiaorg/celestia-node/pull/289) [@Bidon15](https://github.com/Bidon15)
- [node: add the cmdnode.HeadersFlags() to the Bridge Node's init and start commands #390](https://github.com/celestiaorg/celestia-node/pull/390) [@jbowen93](https://github.com/jbowen93)

### BUG FIXES

- [fix(service/header): lazily load Store head #458](https://github.com/celestiaorg/celestia-node/pull/458) [@Wondertan](https://github.com/Wondertan)
- [fix(service/header): allow some clock drift during verification #435](https://github.com/celestiaorg/celestia-node/pull/435) [@Wondertan](https://github.com/Wondertan)
- [service/header: fix ExtendedHeader message duplicates on the network #409](https://github.com/celestiaorg/celestia-node/pull/409) [@Wondertan](https://github.com/Wondertan)
- [fix(header/service): #339 race](https://github.com/celestiaorg/celestia-node/pull/343) [@Wondertan](https://github.com/Wondertan)
- [core: Properly fetch Validators from Core and two more fixes #328](https://github.com/celestiaorg/celestia-node/pull/328) [@Wondertan](https://github.com/Wondertan)
- [header: Added missing `err` value in ErrorW logging calls](https://github.com/celestiaorg/celestia-node/pull/282) [@jbowen93](https://github.com/jbowen93)
- [service/block, node/p2p: Fix race conditions in TestExtendedHeaderBroadcast and TestFull_P2P_Streams.](https://github.com/celestiaorg/celestia-node/pull/288) [@jenyasd209](https://github.com/jenyasd209)
- [ci: increase tokens ratio for dupl to fix false positive scenarios](https://github.com/celestiaorg/celestia-node/pull/314) [@Bidon15](https://github.com/Bidon15)
- [node: Wrap datastore with mutex to prevent data race](https://github.com/celestiaorg/celestia-node/pull/325) [@Bidon15](https://github.com/Bidon15)
- [ci: update Docker entrypoint.sh to use new `store.path` flag name](https://github.com/celestiaorg/celestia-node/pull/337) [@jbowen93](https://github.com/jbowen93)
- [docker: update docker/entrypoint.sh to use new `node.store` flag replacing `store.path` #390](https://github.com/celestiaorg/celestia-node/pull/390) [@jbowen93 ](https://github.com/jbowen93)

### MISCELLANEOUS

- [chore: bump deps #297](https://github.com/celestiaorg/celestia-node/pull/297) [@Wondertan](https://github.com/Wondertan)
- [workflows/lint: update golangci-lint to v1.43 #308](https://github.com/celestiaorg/celestia-node/pull/308) [@Wondertan](https://github.com/Wondertan)
- [feat(node): extract overrides from Config into Settings #292](https://github.com/celestiaorg/celestia-node/pull/292) [@Wondertan](https://github.com/Wondertan)
- [node: fix naming of the test from full to bridge](https://github.com/celestiaorg/celestia-node/pull/341) [@Bidon15](https://github.com/Bidon15)

## v0.1.1 | 2022-01-07
A quick hot-fix release to enable Full Node sync reliably with validator set bigger than 30.

### Bug Fixes
- [Properly fetch Validators from Core and two more fixes #328](https://github.com/celestiaorg/celestia-node/pull/328) [@Wondertan](https://github.com/Wondertan)

## v0.1.0 | 2021-11-03
This is the first `celestia-node` release. The release constitutes the foundation for the data availability "halo" network that will complement the core consensus network.

Mainly, we introduce 3 packages:
* `service/header` - defines everything related to syncing, exchanging, and storing chain headers. 
* `service/share` - defines everything related to sampling and requesting `Shares` (chunks of erasure-coded block data) from the network
* `node` - defines the Node singleton with its assembly, essentially gluing all the bits together.

For further information regarding the architecture and features introduced in this release, refer to the [devnet ADR](https://github.com/celestiaorg/celestia-node/blob/main/docs/adr/adr-001-predevnet-celestia-node.md).

### FEATURES
- [das: Log out square width after sampling #254](https://github.com/celestiaorg/celestia-node/pull/254) [@Wondertan](https://github.com/Wondertan)
- [das: log time it takes to complete as sampling routine for a header #238](https://github.com/celestiaorg/celestia-node/pull/238) [@renaynay](https://github.com/renaynay)
- [node/p2p: Deduplicate PubSub msgs #239](https://github.com/celestiaorg/celestia-node/pull/239) [@Wondertan](https://github.com/Wondertan)
- [service/header: more descriptive err log for append in store.go #231](https://github.com/celestiaorg/celestia-node/pull/231) [@renaynay](https://github.com/renaynay)
- [cmd, service/header: rename genesis to trustedHash to avoid confusion, extract flags to common var for re-use #229](https://github.com/celestiaorg/celestia-node/pull/229) [@renaynay](https://github.com/renaynay)
- [service/header: Verify hash matches header retrieved in RequestByHash() #224](https://github.com/celestiaorg/celestia-node/pull/224) [@renaynay](https://github.com/renaynay)
- [Genesis: The PR to make testnet happen! #221](https://github.com/celestiaorg/celestia-node/pull/221) [@Wondertan](https://github.com/Wondertan)
- [feat(cmd): allow setting log level for start comamnd #217](https://github.com/celestiaorg/celestia-node/pull/217) [@Wondertan](https://github.com/Wondertan)
- [service/header: Header Exchange implemented over Core Node #216](https://github.com/celestiaorg/celestia-node/pull/216) [@renaynay](https://github.com/renaynay)
- [Persist generated key to keystore and reuse existing key if present #213](https://github.com/celestiaorg/celestia-node/pull/216) [@jbowen93](https://github.com/jbowen93)
- [service/header: implement basic syncer #212](https://github.com/celestiaorg/celestia-node/pull/212) [@Wondertan](https://github.com/Wondertan)
- [node: add p2p multiaddr log #204](https://github.com/celestiaorg/celestia-node/pull/204) [@bidon15](https://github.com/bidon15)
- [service/block, service/header: BlockService broadcasts generated ExtendedHeaders to network #203](https://github.com/celestiaorg/celestia-node/pull/203) [@renaynay](https://github.com/renaynay)
- [cmd: add flag to start node on remote core #196](https://github.com/celestiaorg/celestia-node/pull/196) [@vgonkivs](https://github.com/vgonkivs)
- [service/header: Store implementation #195](https://github.com/celestiaorg/celestia-node/pull/195) [@Wondertan](https://github.com/Wondertan)
- [service/header: Implement HeaderExchange #188](https://github.com/celestiaorg/celestia-node/pull/188) [@renaynay](https://github.com/renaynay)
- [das: implement DASer #177](https://github.com/celestiaorg/celestia-node/pull/177) [@Wondertan](https://github.com/Wondertan)
- [share: Availability interface and implementation #171](https://github.com/celestiaorg/celestia-node/pull/171) [@Wondertan](https://github.com/Wondertan)
- [service/share, ipld: adds ability to retrieve shares by namespace ID #170](https://github.com/celestiaorg/celestia-node/pull/170) [@renaynay](https://github.com/renaynay)
- [service/share: Service interface and basic implementation for GetShare #167](https://github.com/celestiaorg/celestia-node/pull/167) [@Wondertan](https://github.com/Wondertan)
- [service/block: add Commit from current block in ExtendedHeader #165](https://github.com/celestiaorg/celestia-node/pull/165) [@renaynay](https://github.com/renaynay)
- [service/share, ipld: adds ability to retrieve shares by namespace ID #160](https://github.com/celestiaorg/celestia-node/pull/160) [@renaynay](https://github.com/renaynay)
- [Add version command #159](https://github.com/celestiaorg/celestia-node/pull/159) [@orlandorode97](https://github.com/orlandorode97)
- [node: start and stop block service by notification from lifecycle#158](https://github.com/celestiaorg/celestia-node/pull/158) [@vgonkivs](https://github.com/vgonkivs)
- [service/header: extension of ExtendedHeader and protobuf serialization #153](https://github.com/celestiaorg/celestia-node/pull/153) [@Wondertan](https://github.com/Wondertan)
- [service/header: Implement basic header Service structure and interfaces #148](https://github.com/celestiaorg/celestia-node/pull/148) [@renaynay](https://github.com/renaynay)
- [node: add cache for blockstore #136](https://github.com/celestiaorg/celestia-node/pull/136) [@vgonkivs](https://github.com/vgonkivs)
- [MAKEFILE: make build #133](https://github.com/celestiaorg/celestia-node/pull/133) [@renaynay](https://github.com/renaynay)
- [ipld: adding IPLD package to provide helpers for dealing with IPLD-based data storage #110](https://github.com/celestiaorg/celestia-node/pull/110) [@renaynay](https://github.com/renaynay)
- [feat(node): unified node constructor #104](https://github.com/celestiaorg/celestia-node/pull/104) [@Wondertan](https://github.com/Wondertan)
- [cmd: main function, full and light commands with subcommands #97](https://github.com/celestiaorg/celestia-node/pull/97) [@Wondertan](https://github.com/Wondertan)
- [service/block: add basic encoding validation in BlockService #96](https://github.com/celestiaorg/celestia-node/pull/96) [@renaynay](https://github.com/renaynay)
- [node: provide DAGService #95](https://github.com/celestiaorg/celestia-node/pull/95) [@Wondertan](https://github.com/Wondertan)
- [node: use Repository on the Node and slightly improve testing utilites #94](https://github.com/celestiaorg/celestia-node/pull/94) [@Wondertan](https://github.com/Wondertan)
- [service/header, service/block: Implements spec'd out types for block and header #93](https://github.com/celestiaorg/celestia-node/pull/93) [@renaynay](https://github.com/renaynay)
- [node: initialization and Repository #86](https://github.com/celestiaorg/celestia-node/pull/86) [@Wondertan](https://github.com/Wondertan)
- [core: initialization and Repository #81](https://github.com/celestiaorg/celestia-node/pull/81) [@Wondertan](https://github.com/Wondertan)
- [service/block: new block listener + erasure coding #77](https://github.com/celestiaorg/celestia-node/pull/77) [@renaynay](https://github.com/renaynay)
- [node: updates for node.Config #74](https://github.com/celestiaorg/celestia-node/pull/74) [@Wondertan](https://github.com/Wondertan)
- [libs/fslock: a simple utility to lock directories #71](https://github.com/celestiaorg/celestia-node/pull/71) [@Wondertan](https://github.com/Wondertan)
- [libs/keystore: introduce Keystore a crypto key manager #66](https://github.com/celestiaorg/celestia-node/pull/66) [@Wondertan](https://github.com/Wondertan)
- [block: create Fetcher interface #59](https://github.com/celestiaorg/celestia-node/pull/59) [@renaynay](https://github.com/renaynay)
- [node: add more p2p services and protocols #57](https://github.com/celestiaorg/celestia-node/pull/57) [@Wondertan](https://github.com/Wondertan)
- [core: package managing Core Node #51](https://github.com/celestiaorg/celestia-node/pull/51) [@Wondertan](https://github.com/Wondertan)
- [rpc: implement simple RPC client to dial Celestia Core endpoints #48](https://github.com/celestiaorg/celestia-node/pull/48) [@renaynay](https://github.com/renaynay)
- [node: Implement basic Node #29](https://github.com/celestiaorg/celestia-node/pull/29) [@Wondertan](https://github.com/Wondertan)