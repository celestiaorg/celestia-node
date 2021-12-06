# CHANGELOG

## v0.1.0 | 2021-11-3
This is the first `celestia-node` release. The release constitutes the foundation for the data availability "halo" network that will complement the core consensus network.

Mainly, we introduce 3 packages:
* `service/header` - defines everything related to headers Data Structures, synchronization logic and p2p exchange logic
* `service/share` - defines everything related to Shares(pieces of shared Block data), the p2p networking logic and sampling
* `node` - keeps central Node singleton with its assembly, essentially glueing all the bits together.

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