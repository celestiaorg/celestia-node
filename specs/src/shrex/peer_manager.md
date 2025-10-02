# Peer Manager Specification for SHREX Protocol

## Abstract

This specification defines the Peer Manager component for the SHREX protocol in the Celestia network. The Peer Manager is responsible for collecting, organizing, validating, and selecting peers for efficient data retrieval operations based on data availability notifications and peer discovery.

## Table of Contents

- [Terminology](#terminology)
- [Overview](#overview)
- [Core Components](#core-components)
- [Protocol Specification](#protocol-specification)
  - [Peer Pools](#peer-pools)
  - [Parameters](#parameters)
  - [Result Types](#result-types)
- [Manager Operations](#manager-operations)
  - [Peer Selection](#peer-selection)
  - [Pool Management](#pool-management)
  - [Validation](#validation)
  - [Garbage Collection](#garbage-collection)
  - [Blacklisting](#blacklisting)
- [References](#references)
- [Requirements Language](#requirements-language)

## Terminology

- **Peer Manager**: The component responsible for collecting, organizing, and providing peers for data retrieval operations
- **Peer Pool**: A collection of peer IDs organized by data hash, storing peers known to have specific data
- **Sync Pool**: A pool with additional metadata including validation status, height, and creation time
- **Validated Pool**: A peer pool that has been confirmed through header subscription to contain legitimate data
- **Cooldown**: A temporary state where a peer is unavailable for selection but not permanently blocked
- **Blacklist**: A permanent block list preventing all future communication with specific peers
- **Discovered Nodes**: Peers found through the discovery service, independent of specific data hashes
- **Initial Height**: The height of the first header received from header subscription, used as a validation baseline
- **Store From**: The biggest height received from header subscription

## Overview

The Peer Manager serves as the central coordination point for peer selection in the SHREX protocol. It aggregates peers from two primary sources:

1. **ShrEx/Sub notifications**: Peers that announce specific data availability through the pubsub system
2. **Discovery service**: Peers found through DHT-based discovery mechanisms

The Peer Manager maintains data-hash-specific pools and a general pool of discovered nodes, enabling efficient peer selection for data retrieval while implementing validation, cooldown, and blacklisting mechanisms to maintain network quality.

## Core Components

### Header Subscription

1. **Subscription Setup**: Manager MUST subscribe to header updates during start
2. **Pool Validation**: Each received header MUST trigger validation of corresponding pool
3. **Height Tracking**:
    - First header sets `initialHeight`
    - Each header updates `storeFrom`

### ShrEx/Sub

1. **Validator Registration**: Manager MUST register as message validator
2. **Message Processing**: All ShrEx/Sub notifications MUST pass through manager validation
3. **Peer Collection**: Valid notifications MUST add peers to appropriate pools

### Discovery

The Peer Manager MUST expose `UpdateNodePool` for discovery:

1. **Peer Addition**: Discovery MUST call `UpdateNodePool` with `isAdded=true` for new peers
2. **Peer Removal**: Discovery MUST call `UpdateNodePool` with `isAdded=false` for removed peers
3. **Blacklist Check**: Blacklisted peers MUST NOT be added to discovered nodes pool

## Protocol Specification

### Peer Pools

The Peer Manager maintains two distinct types of peer collections:

#### Data Hash Pools

- **Purpose**: Store peers organized by specific data hashes they have announced
- **Structure**: Map of data hash strings to sync pools
- **Validation**: Each pool tracks whether its associated data hash has been validated
- **Lifecycle**: Pools are created on-demand when notifications arrive and removed during garbage collection
- **Capacity**: The manager stores pools for the most recent heights only (controlled by `storedPoolsAmount`, default: 10)

#### Discovered Nodes Pool

- **Purpose**: Store peers found through discovery service, independent of specific data
- **Usage**: Fallback option when no data-hash-specific peers are available
- **Management**: Peers are added from discovery and removed on disconnection

### Parameters

The Peer Manager operates with the following configurable parameters:

#### PoolValidationTimeout

- **Type**: Duration
- **Purpose**: Maximum time allowed for a pool to receive validation through header subscription
- **Rationale**: Pools that do not receive corresponding headers within this timeout are considered invalid, and their peers are blacklisted

#### PeerCooldown

- **Type**: Duration
- **Purpose**: Duration a peer remains unavailable after being marked for cooldown
- **Rationale**: Allows temporary removal of unreliable peers without permanent blacklisting

#### GcInterval

- **Type**: Duration
- **Purpose**: Interval between garbage collection cycles
- **Rationale**: Regular cleanup prevents memory growth from outdated or invalid pools

#### EnableBlackListing

- **Type**: Boolean
- **Purpose**: Feature flag to enable or disable peer blacklisting functionality
- **Rationale**: Allows testing and gradual rollout of blacklisting mechanisms

### Result Types

#### ResultNoop

- **Value**: "result_noop"
- **Meaning**: Operation completed successfully with no additional action required
- **Effect**: No state changes in the Peer Manager

#### ResultCooldownPeer

- **Value**: "result_cooldown_peer"
- **Meaning**: Peer should be temporarily unavailable for selection
- **Effect**: Peer is placed on cooldown for the configured duration
- **Use Case**: Temporary issues like timeouts or transient errors

#### ResultBlacklistPeer

- **Value**: "result_blacklist_peer"
- **Meaning**: Peer has misbehaved and should be permanently blocked
- **Effect**: Peer is added to blacklist, disconnected, and blocked from future connections
- **Use Case**: Malicious behavior, invalid data, or protocol violations

## Manager Operations

### Peer Selection

The Peer Manager implements a prioritized peer selection strategy:

#### Selection Priority

1. **First Priority**: Peers from validated data hash pool
   - Peers that have announced the specific data hash being requested
   - Must be from a pool validated through header subscription
   - Provides highest confidence of data availability

2. **Second Priority**: Discovered nodes pool
   - General-purpose peers found through discovery
   - Used when no data-hash-specific peers are available
   - May or may not have the requested data

3. **Blocking Wait**: If no peers available from either source
   - Block until a peer becomes available from either source
   - Return first available peer
   - Subject to context timeout

#### Peer Validation Before Return

Before returning a peer, the manager MUST verify:

- Peer is not blacklisted
- Peer has an active connection
- If validation fails, peer is removed from pool and selection is retried

### Pool Management

#### Pool Creation

- Pools MUST be created lazily when first notification arrives for a data hash
- Each pool MUST store the associated height and creation timestamp
- Pools MUST initialize with unvalidated status

#### Pool Validation

- Pools MUST be validated when a corresponding header arrives from header subscription
- Validation MUST be performed by comparing data hash and height
- Once validated, all peers in the pool MUST be added to discovered nodes pool
- Validation status MUST be atomic to prevent race conditions

#### Pool Storage Limits

- The manager MUST only store pools for recent heights
- The `storeFrom` threshold MUST be updated based on latest header height
- Pools below the threshold MUST be removed during garbage collection
- Default storage depth is 10 most recent heights (`storedPoolsAmount`)

### Validation

The Peer Manager implements the `MessageValidator` interface for ShrEx/Sub:

#### Validation Rules

1. **Self Messages**: Messages from the node itself MUST be accepted without validation

2. **Blacklisted Hash Check**: Messages containing blacklisted hashes MUST be rejected

3. **Blacklisted Peer Check**: Messages from blacklisted peers MUST be rejected

4. **Height Check**: Messages for heights below `storeFrom` threshold MUST be ignored

5. **Peer Collection**: Valid messages MUST result in peer being added to corresponding pool

6. **Discovered Nodes Addition**: If pool is already validated, peer MUST be immediately added to discovered nodes pool

#### Validation Results

- **Accept**: Only for self-originated messages
- **Reject**: For blacklisted peers or hashes
- **Ignore**: For all other cases (valid messages and old heights)

### Garbage Collection

The Peer Manager MUST implement periodic garbage collection:

#### GC Trigger

- Garbage collection MUST run at intervals specified by `GcInterval` parameter
- GC MUST continue until manager shutdown

#### GC Operations

1. **Validated Pool Cleanup**:
   - Remove pools for heights below `storeFrom` threshold
   - Keep recently validated pools

2. **Unvalidated Pool Timeout**:
   - Identify pools older than `PoolValidationTimeout`
   - Pools below `initialHeight` cannot be validated and MUST be removed
   - Timeout pools that should have been validated but were not

3. **Blacklisting**:
   - Timed-out pool data hashes MUST be blacklisted
   - All peers from timed-out pools MUST be collected for blacklisting
   - Blacklisting MUST occur after GC cycle completes

#### Initial Height Requirement

- GC MUST NOT blacklist peers until `initialHeight` is set

### Blacklisting

The Peer Manager implements a blacklisting mechanism for misbehaving peers:

#### Blacklist Reasons

- **reasonMisbehave**: Peer reported as misbehaving through `ResultBlacklistPeer`
- **reasonInvalidHash**: Peer announced data hash that was never validated

#### Blacklist Actions

When blacklisting is enabled:

1. Peer MUST be removed from discovered nodes pool
2. Peer MUST be blocked via connection gater to prevent future connections
3. All existing connections to peer MUST be closed
4. Peer MUST remain blocked permanently (until node restart)

### Connection Management

The Peer Manager MUST monitor peer connectivity:

#### Disconnection Handling

- Manager MUST subscribe to libp2p connectedness events
- When peer disconnects (connectedness becomes `NotConnected`):
  - Peer MUST be removed from discovered nodes pool
  - Peer remains in data hash pools until GC or validation failure
  
#### Connection Validation

- Before returning a peer, manager MUST verify active connection exists
- Disconnected peers MUST be removed from pools and selection retried

## References

1**ShrEx/Sub Specification**: (see shrex-sub.md)
2**Discovery Specification**: (see discovery.md)
3**libp2p Connection Gater**: <https://github.com/libp2p/go-libp2p/blob/master/core/connmgr/gater.go>

## Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.
