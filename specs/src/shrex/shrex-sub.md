# ShrEx/Sub Protocol Specification

## Abstract

**ShrEx/Sub** (Share Exchange/Subscribe) is a push-based notification protocol implementing the publish-subscribe pattern for Extended Data Square (EDS) hash distribution in the Celestia data availability network. The protocol enables efficient dissemination of new EDS availability notifications across different node types.

## Table of Contents

- [Overview](#overview)
- [Node Roles](#node-roles)
- [Message Schema](#message-schema)
- [Protocol Components](#protocol-components)
  - [Pubsub Components](#pubsub-component)
  - [Subscripition Component](#subscription-component)
- [Message Verification](#message-verification)
  - [Validation Interface](#validation-interface)
  - [Validation Pipeline](#validation-pipeline)
- [Component Interaction Flow](#component-interaction-flow)
- [Protocol Behavior](#protocol-behavior)
  - [Publisher Behavior](#publisher-behavior-bn)
  - [Subscriber Behavior](#subscriber-behavior-ln)
- [FloodSub vs GossipSub Rationale](#floodsub-vs-gossipsub-rationale)
  - [Why not GossipSub?](#why-not-gossipsub)
  - [Why FloodSub?](#why-floodsub)
- [Additional Notes](#additional-notes)
- [Links](#links)

## Overview

ShrEx/Sub is built on libp2p's FloodSub router with the following characteristics:

- **Topic ID**: `/eds-sub/0.0.1`
- **Message Distribution**: Flood-based (sends to all connected peers)
- **Overlay**: No mesh topology (unlike GossipSub)

### Node Roles

- **Light Nodes (LN)**: Subscribers that receive EDS notifications for data availability sampling
- **Bridge Nodes (BN)**: Publishers that announce new EDS availability from consensus layer

## Message Schema

The notification message MUST contain the EDS data hash and block height:

```text
Notification {
    data_hash: bytes[32]  // EDS root hash
    height: uint64        // Block height
}
```

**Properties:**

- Messages MUST have a fixed 40-byte payload (32 bytes hash + 8 bytes height)
- Serialization overhead SHOULD be minimal
- Each EDS at a specific height MUST generate only a single notification

## Protocol Components

### PubSub Component

The core publish-subscribe functionality for message distribution:

**Interface Methods:**

```text
// Publish publishes data hash and height to the topic
Publish(context, dataHash []byte, height uint64) -> error

// Subscribe creates a new subscription to the topic
Subscribe() -> (*Subscription, error)
```

**Requirements:**

- Publishers MUST use the topic ID `/eds-sub/0.0.1`
- The Publish method MUST validate input parameters before publishing
- The Subscribe method MUST return a valid subscription or an error

### Subscription Component

The subscription component handles the receiving and processing of ShrEx/Sub notifications:

**Interface Methods:**

```text
// Next blocks the caller until any new EDS DataHash notification arrives.
// Returns only notifications which successfully pass validation.
Next(ctx context.Context) -> (Notification, error) 
```

**Requirements:**

- Implementations MUST maintain an active subscription to `/eds-sub/0.0.1` topic
- Incoming notifications MUST be processed through the validation pipeline
- Only validated notifications SHALL be distributed to registered listeners

## Message Verification

### Validation Interface

**MessageValidator Interface:**

```text
// Validate validates a message from a peer
Validate(context, peerID PeerID, message []byte) -> ValidationResult
```

**ValidationResult Values:**

- `ACCEPT`: Message is valid and MUST be processed
- `REJECT`: Message is invalid and MUST be discarded
- `IGNORE`: Message is valid but duplicate/stale and SHOULD be ignored

### Validation Pipeline

ShrEx/Sub implementations MUST implement the following validation process:

**Format Validation Requirements:**

- Hash length MUST be exactly 32 bytes
- Hash MUST NOT be all zeros
- Height MUST be a valid block height
- Message size MUST NOT exceed protocol limits

### Component Interaction Flow

```text
1. PubSub receives message from network
2. Subscription processes message through validation
3. Validated messages sent to registered listeners
```

## Protocol Behavior

### Publisher Behavior (BN)

- **Trigger**: Publishers MUST publish when a new EDS becomes available locally
- **Action**: Publishers MUST publish EDS hash and height to `/eds-sub/0.0.1` topic
- **Frequency**: Each EDS MUST be published exactly once per height
- **Validation**: Publishers MUST only publish hashes for locally validated and available EDS

### Subscriber Behavior (LN)

- **Subscription**: Subscribers MUST maintain an active subscription to `/eds-sub/0.0.1`
- **Processing**: Subscribers MUST validate received hash format and height
- **Action**: Subscribers MUST process notifications through registered listeners
- **Deduplication**: Subscribers MUST ignore duplicate notifications

## FloodSub vs GossipSub Rationale

### Why not GossipSub?

In celestia-node, we extensively use libp2p's GossipSub router, which provides bandwidth-efficient yet secure message dissemination over Celestia's DA p2p network. However, it is not well suited for exchanging recent EDS notifications.

`GossipSub`'s efficacy comes from an overlay mesh network based on "physical" connections. Peers form logical links and every gossip is *pushed* only to these peers in the mesh. A new logical link is established on every new "physical" connection. When there are too many logical links (>DHi), random logical links are pruned. However, there is no differentiation between peer types so pruning can happen to any peer.

`GossipSub` implements peer exchange with pruned peers - when BN has too many links, it may prune an LN and then send it a bunch of peers that are not guaranteed to be BNs. Therefore, the LN can end up isolated with other LNs from new EDS notifications.

### Why FloodSub?

FloodSub, on the other hand, sends messages to every "physical" connection without overlay mesh of logical links, which solves the problem with the cost of higher message duplication factor on the network. Although, a moderate amount of duplicates from different peers are helpful in this case. If the primary message sender peer does not serve data, the senders of duplicates are requested instead.

**Trade-offs:**

- **Higher Message Duplication**: Each message sent to every connection
- **Bandwidth Cost**: ~40 bytes × peer_count per notification (32 bytes hash + 8 bytes height)
- **Acceptable**: Given small message size and notification frequency

## Additional Notes

Besides *pushing* gossip, GossipSub has an internal *lazy-push* mechanism. Randomly connected peers outside the overlay mesh are selected and sent `IHAVE` messages (hash of the actual message) and can respond with `IWANT`. In the case of an isolated LN, there is a chance that it will still receive the data via the lazy-pull mechanism; however, it is randomized, and thus the isolated node can miss notifications.

We could increase GossipFactor to 1, which means always sending `IHAVE` to every connected peer outside the overlay mesh. However, the notification message is a hash, and there is no reason to pull the hash by its hash compared to a direct push of the hash.

## Links

- [libp2p PubSub Overview](<https://github.com/libp2p/specs/blob/master/pubsub/README.md>)
- [Shrex-Sub Implementation](<https://github.com/celestiaorg/celestia-node/tree/main/share/shwap/p2p/shrex/shrexsub>)

## Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.
