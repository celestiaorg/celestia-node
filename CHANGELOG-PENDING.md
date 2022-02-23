# Unreleased Changes

## vX.Y.Z

Month, DD, YYYY

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
