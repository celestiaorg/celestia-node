# ADR for Devnet Celestia Node

## Authors

@renaynay @Wondertan @liamsi

## Changelog

* 2021-19-08: initial draft

## Context

This ADR describes a basic pre-devnet design for a "Celestia Node" that was decided at the August 2021 Kyiv offsite that will ideally be completed in early November 2021 and tested in the first devnet.

The goal of this design is to get a basic structure of "Celestia Node" interoperating with a "Celestia Core" consensus node by November 2021 (devnet). 

After basic interoperability on devnet, there will be an effort to merge consensus functionality into the "Celestia Node" design as a modulor service that can be added on top of the basic functions of a "Celestia Node".

## Decision

A "Celestia Node" will be distinctly different than a "Celestia Core" node in the initial implementation for devnet, with plans to merge consensus core functionality into the general design of a "Celestia Node", just added as an additional `ConsensusService` on the node.

For devnet, we require two modes for the Celestia Node: `light` and `full`, where the `light` node performs data availability sampling and the `full` node processes, stores, and serves new blocks from Celestia Core consensus nodes.
 
**For devnet, a `light` Celestia Node must be able to do the following:**
* propagate relevant block information (in the form of `ExtendedHeader`s and `ErasureCodingFraudProof`s to its "Celestia Node" peers
* verify `ExtendedHeader`s
* perform and serve sampling and `SharesByNamespace` requests
* request and serve `State` to get `AccountBalance` in order to submit transactions


**For devnet, a `full` Celestia Node must be able to do everything a `light` Celestia Node does, in addition to the following:**
* receive "raw" (un-erasure coded) blocks from a "Celestia Core" node by subscribing to `NewBlockEvents` using the `/block` RPC endpoint of "Celestia Core" node
* erasure code the block / verify erasure coding
* create an `ExtendedHeader` with the raw block header, the generated `DataAvailabilityHeader` (DAH), as well as the `ValidatorSet` and serve this `ExtendedHeader` to the Celestia network
* request "raw" blocks from other `full` Celestia Nodes on the network

![predevnet flow](./img/predevnetnode.png)

## Detailed Design

The Celestia Node will have a central `Node` data structure around which all services will be focused. A user will be able to initialise a Celestia Node as either a `full` or `light` node. Upon start, the `Node` gets created, configured and will also configure services to register on the `Node` based on the node type (`light` or `full`).

A `full` node encompasses the functionality of a `light` node along with additional services that allow it to interact with a Celestia Core node.

A `light` node will provide the following services: 
* `ExtendedHeaderService`
    * `ExtendedHeaderExchange`
    * `ExtendedHeaderSub`
    * `ExtendedHeaderStore`
* `FraudProofService` *(optional for devnet)*
    * `FraudProofSub`
    * `FraudProofStore`
* `ShareService`
    * `ShareExchange`
    * `ShareStore`
* `StateService` *(optional for devnet)*
    * `StateExchange`
* `TransactionService` *(dependent on `StateService` implementation, but optional for devnet)*
    * `SubmitTx`

A `full ` node will provide the following services: 
* `ExtendedHeaderService`
    * `ExtendedHeaderExchange`
    * `ExtendedHeaderVerification` (`light` nodes only)
    * `ExtendedHeaderSub`
    * `ExtendedHeaderStore`
* `FraudProofService` *(optional for devnet)*
    * **`FraudProofGeneration`**
    * `FraudProofSub`
    * `FraudProofStore`
* `ShareService`
    * `ShareExchange`
    * `ShareStore`
* `BlockService`
    * `BlockErasureCoding` 
    * `NewBlockEventSubscription` (`full` node <> `Celestia Core` node)
    * `BlockExchange` (`full` node <> `full` node)
    * `BlockStore`
* `StateService` *(optional for devnet)*
    * `StateExchange`
* `TransactionService` *(dependent on `StateService` implementation, but optional for devnet)*
    * `SubmitTx`\


For devnet, it should be possible for Celestia `full` Nodes to receive information directly from Celestia Core nodes or from each other. 

## Considerations 

### State Fraud Proofs
For the Celestia Node to be able to propagate `StateFraudProof`s, we must modify Celestia Core to store blocks with invalid state and serve them to both the Celestia Node and the Celestia App, **and** the Celestia App must be able to generate and serve `StateFraudProof`s via RPC to Celestia nodes.

This feature is not necessarily required for devnet (so state exection functionality for Celestia Full Nodes can be stubbed out), but it would be nice to have for devnet as we will likely allow Celestia Full Nodes to speak with other Celestia Full Nodes instead of running a trusted Celestia Core node simultaenously and relying on it for information.

A roadmap to implementation could look like the following: 

The Celestia Full Node would adhere to the ABCI interface in order to communicate with the Celestia App (similar to the way Optimint does it). The Celestia Full Node would send State requests to the Celestia App in order for the Celestia app to replay the transactions in the block and verify the state. 

For devnet, it is okay to stub out state verification functionality. For example, a Celestia Full Node would download reserve transactions, but not replay them. 


### Fraud Proofs

For validators (Celestia Core) to be able to sign blocks, they need to erasure code the block data and commit to the data root of the erasure coded block that is stored in the header. The validators do not need to store the erasure coded block, but only the "raw" block with the data root in the header. A Celestia Node will fetch the "raw" block from Celestia Core via the `/block` endpoint, and then erasure code the block, checking it against the data root in the header, and then store the erasure coded block.

For devnet, the Celestia Node will not be able to generate state fraud proofs as it will not have a state execution environment, so it will rely on Celestia Core to produce state fraud proofs and retrieve the proof via a `/state_fraud` endpoint. The Celestia Node will also require the "state fraudulent" block, so Celestia Core needs to make those blocks available via the `/block` endpoint.

### Light nodes serving shares and samples

At the moment, we will be using [bitswap](https://github.com/ipfs/go-bitswap) to retrieve samples and shares from the network. The way Bitswap works requires nodes that have the requested data to serve it. This is not necessarily ideal for a "light node" to do as supporting serving samples/shares would expand the resource requirements for a light node.

In the future, we should consider moving away from bitswap to either [GraphSync](https://github.com/ipfs/go-graphsync) or a custom protocol. 

## Consequences

While this design is not ideal, it will get us to a devnet more quickly and allow us to iterate rather than try to design and implement the perfect Celestia node from the start. 

**Positive**

Iterative process will allow us to test out non-p2p-related functionality much sooner, and will allow us to incrementally approach debugging rather than trying to get the design/implementation perfect in one go.

**Negative**

* We will end up throwing out a lot of the implementation work we do now since our eventual goal is to merge consensus functionality into the concept of a "Celestia Node". 
* The current design requires erasure coding to be done twice and stores data twice (raw block in Celestia Core and erasure coded block in Celestia Node) which is redundant and should be consolidated in the future.


## Status

Proposed

