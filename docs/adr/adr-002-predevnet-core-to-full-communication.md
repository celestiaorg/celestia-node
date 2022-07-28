# ADR #002: Devnet Celestia Core <> Celestia Node Communication

## Authors

@renaynay @Wondertan

## Changelog

* 2021-09-09: initial draft

## Legend

**Celestia Core** = tendermint consensus node that lives in [the celestia-core repository](https://github.com/celestiaorg/celestia-core).

**Celestia Node** = celestia `full` or `light` nodes that live in [this repository](https://github.com/celestiaorg/celestia-node).

## Context

After the offsite, there was a bit of confusion on what the default behaviour for running a Celestia Full node should be in the devnet. Since we decided on an architecture where Core nodes will communicate with Celestia Full nodes *exclusively* via RPC (over an HTTP connection, for example), it is necessary that a certain percentage of Celestia Full nodes in the devnet run with either an embedded Core node process or are able to fetch block information from a remote Core endpoint, otherwise there would be no way for the two separate networks (Core network and Celestia network) to communicate.

## Decision

Since the flow of information in devnet is unidirectional, where Core nodes provide block information to Celestia Full nodes, the default behaviour for running a Celestia Full node is to have an embedded Core node process running within the Full node itself. Not only will this ensure that at least some Celestia Full nodes in the network will be communicating with Core nodes, it also makes it easier for end users to spin up a Celestia Full node without having to worry about feeding the Celestia Full node a remote Core endpoint from which it would fetch information.

It is also important to note that for devnet, it should also be possible to run Celestia Full nodes as `standalone` processes (without a trusted remote or embedded Core node) as Celestia Full nodes should also be capable of learning of block information on a P2P-level from other Celestia Full nodes.

## Detailed Design

* Celestia Full node should be able to run in a `default` mode where Celestia Full node embeds a Core node process
* Celestia Full node should also be able to be started with a `--core.disable` flag to indicate that the Celestia Full node will be running *without* a Core node process
* Celestia Full node should be able to take in a `--core.remote` endpoint that would indicate to the Full node that it should *not* embed the Core node process, but rather dial the provided remote Core node endpoint.
* Celestia Full nodes that rely on Core node processes (whether embedded or remote) should also communicate with other Celestia Full nodes on a P2P-level, broadcasting new headers from blocks that they've fetched from the Core nodes *and* being able to handle broadcasted block-related messages from other Full nodes on the network.

It is preferable that a devnet-ready Celestia Full node is *agnostic* to the method by which it receives new block information. Therefore, we will abstract the interface related to "fetching blocks" so that in the view of the Celestia Full node, it does not care *how* it is receiving blocks, only that it *is* receiving new blocks.

## Consequences of embedding Celestia Core process into Celestia Full node

### Positive

* Better UX for average devnet users who do not want to deal with spinning up a Celestia Core node and passing the endpoint to the Celestia Full node.
* Makes it easier to guarantee that there will be *some* Full nodes in the devnet that will be fetching blocks from Celestia Core nodes.

### Negative

* Eventually this work will be rendered useless as communicating with Celestia Core over RPC is a crutch we decided to use in order to streamline interoperability between Core and Full nodes. All communication beyond devnet will be over the P2P layer.

## Status

Proposed
