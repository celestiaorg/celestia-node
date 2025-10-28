# Peer Manager Specification for SHREX Protocol

## Abstract

This specification defines the Peer Manager component for the SHREX protocol in the Celestia network. The Peer Manager collects, organizes, and provides peers for efficient data retrieval based on data availability notifications and peer discovery.

## Overview

The Peer Manager serves as the central coordination point for peer selection in the SHREX protocol. It aggregates peers from two primary sources:

1. **ShrEx/Sub notifications**: Peers announcing specific data availability through pubsub
2. **Discovery service**: Peers found through DHT-based discovery

The manager maintains hash pools by creating a pool for each data hash when first announced via ShrEx/Sub, then validates these pools by matching them against headers received from its header subscription (representing the node's subjective view of the chain). Once a pool is validated through header matching, all peers in that pool are promoted to the discovered nodes pool. Pools that are not validated within a timeout are garbage collected. This validation mechanism ensures the manager provides the best request routing possible

## Flowchart: Adding Peers to the Manager

```text
                    ┌─────────────────────────────┐
                    │   Peer Announcement Source  │
                    └──────────┬──────────────────┘
                               │
              ┌────────────────┴────────────────┐
              │                                 │
              ▼                                 ▼
   ┌─────────────────────┐         ┌─────────────────────┐
   │  ShrEx/Sub Protocol │         │  Discovery Service  │
   │  Notification        │        │  Event              │
   └──────────┬───────────┘        └──────────┬──────────┘
              │                               │
              │                               │
              ▼                               ▼
   ┌─────────────────────┐         ┌─────────────────────┐
   │  Validator Function │         │  Add directly to    │
   │  - Self check       │         │  Discovered Nodes   │
   │  - Deduplication    │         │  Pool               │
   │  - Filtering        │         └─────────────────────┘
   └──────────┬───────────┘
              │
              ▼
      ┌───────────────┐
      │  Valid?       │
      └───┬───────────┘
          │
    ┌─────┴─────┐
    │           │
   No          Yes
    │           │
    ▼           ▼
 ┌─────┐   ┌─────────────────────┐
 │Reject│  │ Add to Hash-Specific│
 └─────┘   │ Pool                │
           └──────────┬──────────┘
                      │
                      ▼
           ┌─────────────────────┐
           │ Check: Is pool      │
           │ already validated?  │
           └──────────┬──────────┘
                      │
              ┌───────┴───────┐
              │               │
             No              Yes
              │               │
              ▼               ▼
    ┌─────────────────┐   ┌─────────────────────┐
    │ Wait for header │   │ Promote peer to     │
    │ validation      │   │ Discovered Nodes    │
    └─────────────────┘   │ Pool                │
                          └─────────────────────┘
           

    ┌──────────────────────────────────────────┐
    │     Header Subscription (Parallel)       │
    └──────────────────┬───────────────────────┘
                       │
                       ▼
            ┌─────────────────────┐
            │ Receive validated   │
            │ header              │
            └──────────┬──────────┘
                       │
                       ▼
            ┌─────────────────────┐
            │ Match header hash   │
            │ with pools          │
            └──────────┬──────────┘
                       │
                       ▼
            ┌─────────────────────┐
            │ Mark pool as        │
            │ validated           │
            └──────────┬──────────┘
                       │
                       ▼
            ┌─────────────────────┐
            │ Promote all peers   │
            │ from pool to        │
            │ Discovered Nodes    │
            └─────────────────────┘
```

## Core Components

### Header Subscription

The manager MUST subscribe to header updates to validate peer pools:

- Pools are validated by matching announced data hashes against headers that have been validated and added as part of the node's subjective view of the chain
- The manager MUST track the initial header height and maintain a threshold for valid pools
- The manager tracks the initial header height (the first header received after start-up, representing the node's subjective head at initialization)
- The manager maintains a rolling validation window that keeps pools for approximately the most recent 10 headers, ignoring announcements for older heights
- Only pools validated through this header matching process are considered valid

### ShrEx/Sub Integration

The manager MUST act as a message validator for ShrEx/Sub:

- All incoming data availability notifications MUST be validated
- Peers MUST be added to data-hash-specific pools based on valid notifications
- If the data hash pool is already validated, the peer MUST be immediately added to discovered nodes pool as well

#### Notification Handling

The manager uses a validator function to process incoming notifications from shrex-sub and perform deduplication:

- Notifications from the node itself are accepted
- Duplicate or outdated notifications are filtered out to maintain data freshness
- Valid notifications result in peers being added to data-hash-specific pools
- If the data hash pool is already validated, the peer is immediately promoted to the discovered nodes pool

### Discovery Integration

The manager MUST provide an `UpdateNodePool` interface for the discovery service:

- Discovery MUST add newly found peers via `UpdateNodePool(peerID, isAdded=true)`
- Discovery MUST remove disconnected peers via `UpdateNodePool(peerID, isAdded=false)`

## Peer Pools

### Data Hash Pools

Peers are organized by the data hashes they announce:

- **Creation**: Pools MUST be created when ShrEx/Sub notifications arrive for a data hash
- **Population**: Peers MUST be added to pools when they announce specific data hashes via ShrEx/Sub
- **Validation**: Pools MUST track validation status until a matching header arrives from header subscription
- **Promotion**: When a pool becomes validated (or is already validated), peers MUST be added to the discovered nodes pool
- **Storage**: Only recent pools MUST be kept (based on configurable depth from latest header)

### Discovered Nodes Pool

A general pool of peers available for data retrieval:

- **From Discovery**: Peers found through the discovery service MUST be added directly
- **From Validated Pools**: Peers from data hash pools MUST be promoted here once their pool is validated

## Peer Selection

The manager MUST implement prioritized peer selection:

1. **First Priority**: Peers from validated data-hash-specific pools
2. **Second Priority**: Peers from discovered nodes pool
3. **Blocking**: Wait for peers if none available (subject to context timeout)

## Validation and Quality Control

### Pool Validation

- Pools MUST be validated through header subscription within a configurable timeout
- Validated pools indicate their peers likely have legitimate data
- Peers from validated pools MUST be added to the discovered nodes pool

### Result Types

Operations return one of three results to callers:

- **ResultNoop**: Operation completed successfully, no action needed
- **ResultCooldownPeer**: Peer temporarily unavailable (transient issues)

### Optional Blacklisting

While not currently active in production, the manager has infrastructure for peer blacklisting that may be enabled in the future. This would allow for permanent exclusion of misbehaving peers based on:

- Peers reported through future result codes indicating malicious behavior
- Peers that announced data hashes which were never validated within the timeout period

This functionality is designed but not currently enabled, allowing for future network hardening as needed.

### Garbage Collection

Periodic cleanup MUST remove:

- Validated pools for heights below the storage threshold
- Unvalidated pools that exceed the validation timeout
- Disconnected peers from the discovered nodes pool

## Parameters

- **PoolValidationTimeout**: Maximum time for pool to receive validation
- **PeerCooldown**: Duration peer remains unavailable after cooldown
- **GcInterval**: Time between garbage collection cycles
- **EnableBlackListing**: Feature flag for blacklisting functionality
- **StoredPoolsAmount**: Number of recent height pools to maintain (default: 10)

## Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.
