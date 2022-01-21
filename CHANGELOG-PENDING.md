# Unreleased Changes

## vX.Y.Z

Month, DD, YYYY

### BREAKING CHANGES

- [chore: rename Repository to Store #296](https://github.com/celestiaorg/celestia-node/pull/296) [@Wondertan](https://github.com/Wondertan)
- [chore: rename Full node to Bridge node #294](https://github.com/celestiaorg/celestia-node/pull/294) [@Wondertan](https://github.com/Wondertan)
- [node: remove InitWith #291](https://github.com/celestiaorg/celestia-node/pull/291) [@Wondertan](https://github.com/Wondertan)

### FEATURES

- [feat(cmd): give a birth to cel-shed and p2p key utilities #281](https://github.com/celestiaorg/celestia-node/pull/281) [@Wondertan](https://github.com/Wondertan)
- [feat(cmd|node): MutualPeers Node option and CLI flag #280](https://github.com/celestiaorg/celestia-node/pull/280) [@Wondertan](https://github.com/Wondertan)
- [node: enhance DI allowing overriding of dependencies](https://github.com/celestiaorg/celestia-node/pull/290) [@Wondertan](https://github.com/Wondertan)

### IMPROVEMENTS

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

### BUG FIXES

- [fix(header/service): #339 race](https://github.com/celestiaorg/celestia-node/pull/343) [@Wondertan](https://github.com/Wondertan)
- [core: Properly fetch Validators from Core and two more fixes #328](https://github.com/celestiaorg/celestia-node/pull/328) [@Wondertan](https://github.com/Wondertan)
- [header: Added missing `err` value in ErrorW logging calls](https://github.com/celestiaorg/celestia-node/pull/282) [@jbowen93](https://github.com/jbowen93)
- [service/block, node/p2p: Fix race conditions in TestExtendedHeaderBroadcast and TestFull_P2P_Streams.](https://github.com/celestiaorg/celestia-node/pull/288) [@jenyasd209](https://github.com/jenyasd209)
- [ci: increase tokens ratio for dupl to fix false positive scenarios](https://github.com/celestiaorg/celestia-node/pull/314) [@Bidon15](https://github.com/Bidon15)
- [node: Wrap datastore with mutex to prevent data race](https://github.com/celestiaorg/celestia-node/pull/325) [@Bidon15](https://github.com/Bidon15)
- [ci: update Docker entrypoint.sh to use new `store.path` flag name](https://github.com/celestiaorg/celestia-node/pull/337) [@jbowen93](https://github.com/jbowen93)

### MISCELLANEOUS

- [chore: bump deps #297](https://github.com/celestiaorg/celestia-node/pull/297) [@Wondertan](https://github.com/Wondertan)
- [workflows/lint: update golangci-lint to v1.43 #308](https://github.com/celestiaorg/celestia-node/pull/308) [@Wondertan](https://github.com/Wondertan)
- [feat(node): extract overrides from Config into Settings #292](https://github.com/celestiaorg/celestia-node/pull/292) [@Wondertan](https://github.com/Wondertan)
- [node: fix naming of the test from full to bridge](https://github.com/celestiaorg/celestia-node/pull/341) [@Bidon15](https://github.com/Bidon15)
