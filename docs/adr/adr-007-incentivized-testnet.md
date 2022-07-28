# ADR #007: Incentivized Technical Requirements ADR

## Changelog

* 2022-04-15: proposed

## Authors

@renaynay @Wondertan

## Context

This ADR serves as a tracking document for celestia-node's technical requirements for the planned incentivized testnet.

All epics/tracking issues have been linked in the sub-headers of [technical requirements](#technical-requirements).

Top-level features will include a link to the epic-style tracking issue as well which can be used for a further detailed
breakdown of the individual feature requirements.

## Legend

* **DA node**: any node type implemented in celestia-node
* **DA network**: the p2p network of celestia-node

## Technical Requirements

### [`DASer` hardening and improvements](https://github.com/celestiaorg/celestia-node/issues/632)

#### [`DASState`](https://github.com/celestiaorg/celestia-node/issues/427)

In order to expose the state of the `daser` and make it accessible via the public API, there needs to be a mechanism to
track the state of the `daser` in a similar way to how `SyncState` does it. `DASState` will capture basic information
about the state of the `daser`, such as the latest sampled header.

#### [Share cache](https://github.com/celestiaorg/celestia-node/issues/180)

The `daser`'s current implementation is very basic in that it performs two sampling routines:

1. samples over latest `ExtendedHeader`s received via `headersub`
2. samples over all `ExtendedHeader`s between the last `DAS`ed header (checkpoint) and the latest `ExtendedHeader` in
   the network

The issue with the above approach is that there is a possibility that the `daser` would sample over an `ExtendedHeader`
which it has already sampled. The only way to keep track of which heights have already been sampled is to implement a
share cache that would cache the results of sampling processes on disk in such a way that tracks which heights have been
sampled and whether it was successful or not.

#### [Better error handling](https://github.com/celestiaorg/celestia-node/issues/554)

The catch-up routine of the `DASer` currently exits the catch-up routine upon error and logs out an instruction to
restart the node in order to continue sampling. Instead, there should be retry logic and potentially the graceful
shutdown of `DASer` and potentially node.

### Public API

The public API is currently being drafted [here](https://github.com/celestiaorg/celestia-node/pull/506/files). There are
several components that will go into implementing the overall design as well as the individual module implementations.

### [Bad Encoding Fraud Proof](https://github.com/celestiaorg/celestia-node/issues/528) (continuation)

The full implementation of bad encoding fraud proof (BEFP) generation, propagation and handling is required for the
incentivized testnet. At the moment, generation and broadcasting of BEFPs is still in review.

### [Testing](https://github.com/celestiaorg/celestia-node/issues/7)

One of the biggest priorities for incentivized testnet is to test a lot of the assumptions we are working under in terms
of sampling in realistic network conditions with a variety of topologies.

Among the most important test cases, the following are required:

* [Large Block Tests](https://github.com/celestiaorg/celestia-node/issues/602)
* [Block Recovery Tests](https://github.com/celestiaorg/test-infra/issues/21)
* [Peer Discovery Tests](https://github.com/celestiaorg/celestia-node/issues/649)

In terms of testing infrastructure, [testground](https://github.com/testground/testground) will be considered. A [proof of concept](https://github.com/celestiaorg/celestia-node/issues/638) will be done in order to accomplish the above tests.

### [Wallet Management](https://github.com/celestiaorg/celestia-node/issues/415)

All DA node types will support sending transactions and querying state-related information, meaning DA nodes will need
an account. So far, the standard key utility from cosmos-SDK has been made available via CLI for all node types, however
more robust key/wallet management via the node's public API is required.

### [`HeaderExchange` extensions and hardening](https://github.com/celestiaorg/celestia-node/issues/497)

The current implementation of `HeaderExchange` has to be improved and hardened as there is a hard dependency on trusted
peers at the moment. Some high-level improvement requirements are:

* Read/write deadlines on libp2p streams
* Server-side request limits
* Server-side maximum request size configuration
* Adjacency of a batched `ExtendedHeader` response must be verified in `HeaderEx` upon receiving the response
* Request retries
* Cycling peers for requests

More detail can be found in the issue linked in the header.

### IPLD package improvements

* [Drop IPFS plugin support](https://github.com/celestiaorg/celestia-node/issues/656)
* [NMT/IPLD optimization](https://github.com/celestiaorg/celestia-node/issues/614)

### [Telemetry](https://github.com/celestiaorg/celestia-node/issues/260)

All node types must have metrics collection and tracing of hot paths implemented.

## Nice to have

### [Disk usage/storage optimizations](https://github.com/celestiaorg/celestia-node/issues/671)

* [Badgerv3 update](https://github.com/celestiaorg/celestia-node/issues/482)
* [Share and header pruning](https://github.com/celestiaorg/celestia-node/issues/272)
