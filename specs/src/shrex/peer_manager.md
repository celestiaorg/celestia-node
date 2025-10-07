# Peer Manager Specification for SHREX Protocol

## Abstract

This specification defines the Peer Manager component for the SHREX protocol in the Celestia network. The Peer Manager collects, organizes, and provides peers for efficient data retrieval based on data availability notifications and peer discovery.

## Overview

The Peer Manager serves as the central coordination point for peer selection in the SHREX protocol. It aggregates peers from two primary sources:

1. **ShrEx/Sub notifications**: Peers announcing specific data availability through pubsub
2. **Discovery service**: Peers found through DHT-based discovery

The manager maintains data-hash-specific pools and validates them against headers from header subscription, implementing mechanisms to ensure network quality.

## Core Components

### Header Subscription

The manager MUST subscribe to header updates to validate peer pools:

- Pools MUST be validated by matching received headers against announced data hashes
- The manager MUST track the initial header height and maintain a threshold for valid pools
- Only pools validated through header subscription are considered trusted

### ShrEx/Sub Integration

The manager MUST act as a message validator for ShrEx/Sub:

- All incoming data availability notifications MUST be validated
- Peers MUST be added to data-hash-specific pools based on valid notifications
- If the data hash pool is already validated, the peer MUST be immediately added to discovered nodes pool
- Notifications from blacklisted peers or containing blacklisted hashes MUST be rejected

### Discovery Integration

The manager MUST provide an `UpdateNodePool` interface for the discovery service:

- Discovery MUST add newly found peers via `UpdateNodePool(peerID, isAdded=true)`
- Discovery MUST remove disconnected peers via `UpdateNodePool(peerID, isAdded=false)`
- Blacklisted peers MUST NOT be added to the discovered nodes pool

## Peer Pools

### Data Hash Pools

Peers are organized by the data hashes they announce:

- **Creation**: Pools MUST be created when ShrEx/Sub notifications arrive for a data hash
- **Population**: Peers MUST be added to pools when they announce specific data hashes via ShrEx/Sub
- **Validation**: Pools MUST track validation status until a matching header arrives from header subscription
- **Promotion**: When a pool becomes validated (or is already validated), peers MUST be added to the discovered nodes pool
- **Storage**: Only recent pools MUST be kept (based on configurable depth from latest header)
- **Cleanup**: Unvalidated pools that timeout MUST have their data hash and peers blacklisted

### Discovered Nodes Pool

A general pool of peers available for data retrieval:

- **From Discovery**: Peers found through the discovery service MUST be added directly
- **From Validated Pools**: Peers from data hash pools MUST be promoted here once their pool is validated
- **Removal**: Peers MUST be removed when they disconnect or are blacklisted

## Peer Selection

The manager MUST implement prioritized peer selection:

1. **First Priority**: Peers from validated data-hash-specific pools
2. **Second Priority**: Peers from discovered nodes pool
3. **Blocking**: Wait for peers if none available (subject to context timeout)

Before returning any peer, the manager MUST verify the peer is not blacklisted and has an active connection.

## Validation and Quality Control

### Pool Validation

- Pools MUST be validated through header subscription within a configurable timeout
- Validated pools indicate their peers likely have legitimate data
- Peers from validated pools MUST be added to the discovered nodes pool
- Unvalidated pools that timeout MUST result in blacklisting their data hash and peers

### Result Types

Operations return one of three results to callers:

- **ResultNoop**: Operation completed successfully, no action needed
- **ResultCooldownPeer**: Peer temporarily unavailable (transient issues)
- **ResultBlacklistPeer**: Peer permanently blocked (malicious behavior)

### Blacklisting

When enabled, the manager blacklists peers for:

- **Misbehavior**: Reported through `ResultBlacklistPeer`
- **Invalid announcements**: Announced data that never validated

Blacklisted peers MUST be disconnected, blocked from future connections, and removed from all pools.

### Garbage Collection

Periodic cleanup MUST remove:

- Validated pools for heights below the storage threshold
- Unvalidated pools that exceed the validation timeout
- Disconnected peers from the discovered nodes pool

Peers from timed-out pools MUST be blacklisted to maintain network quality.

## Parameters

- **PoolValidationTimeout**: Maximum time for pool to receive validation
- **PeerCooldown**: Duration peer remains unavailable after cooldown
- **GcInterval**: Time between garbage collection cycles
- **EnableBlackListing**: Feature flag for blacklisting functionality
- **StoredPoolsAmount**: Number of recent height pools to maintain (default: 10)

## Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.
