# Discovery Protocol Specification

## Abstract

This specification defines the peer discovery protocol for the data availability network. The discovery protocol enables efficient peer discovery and connection establishment for nodes in the network, allowing them to find peers with specific capabilities through tag-based discovery.

## Table of Contents

1. [Introduction](#introduction)
2. [Requirements Language](#requirements-language)
3. [Terminology](#terminology)
4. [Overview](#overview)
5. [Architecture](#architecture)
6. [Protocol Specification](#protocol-specification)
7. [API Reference](#api-reference)
8. [Implementation Details](#implementation-details)
9. [References](#references)

## Introduction

The Discovery protocol is a foundational component of the DA network that facilitates peer discovery and connection establishment. This protocol enables nodes to discover and establish communication channels with other peers based on their capabilities and roles within the network.

The discovery mechanism operates on tag-based discovery, where peers advertise their presence under specific tags and other peers discover them through DHT-based routing.

### Scope

This specification covers:

- Peer discovery mechanisms for DA network
- Tag-based peer advertisement and lookup
- Integration with node types
- Bounded peer set management

## Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.

## Terminology

- **Discovery**: The main service that handles peer discovery operations
- **Tag**: A string identifier that categorizes peers by their capabilities (e.g., "full", "archival")
- **Limited Set**: A bounded collection that maintains discovered peers with configurable size limits
- **Advertisement**: The process of announcing peer presence under a specific tag
- **Parameters**: Configuration structure containing discovery settings

## Overview

The Discovery protocol operates as a tag-based discovery system where:

1. **Full nodes (FN) and Bridge nodes (BN)** advertise their presence under specific tags
2. **All nodes** (Full, Bridge, Light) can discover other peers associated with tags of interest
3. **Light nodes (LN)** primarily use discovery to find Full and Bridge nodes
4. **Limited Set** maintains a bounded collection of discovered peers

The protocol enables communication establishment for higher-level protocols. Light nodes discover Full and Bridge nodes to access data availability services, while Full and Bridge nodes advertise their availability to be discoverable by other nodes in the network.

### Node Type Usage Patterns

- **Full Nodes**: Advertise under tags AND discover other peers
- **Bridge Nodes**: Advertise under tags AND discover other peers
- **Light Nodes**: Only discover peers (do not advertise)

## Architecture

### Component Overview

```pgsql
┌─────────────────────────────────────────────────────────────┐
│                Discovery Service                            │
├─────────────────────┬───────────────────┬───────────────────┤
│      Discovery      │    Limited Set    │   Peer Updates    │
│    (Advertise)      │   (set.Size())    │   (Callbacks)     │
├─────────────────────┼───────────────────┼───────────────────┤
│                DHT Routing Discovery                        │
├─────────────────────────────────────────────────────────────┤
│                    P2P Host                                 │
└─────────────────────────────────────────────────────────────┘
```

### Core Components

1. **Discovery**: Main service that handles advertisement and peer discovery operations
2. **Limited Set**: Bounded collection that maintains discovered peers (accessed via `set.Size()`)
3. **Parameters**: Configuration including `PeersLimit`, `AdvertiseInterval`
4. **Peer Update Callbacks**: Event handlers for peer addition/removal notifications via `WithOnPeersUpdate`

## Protocol Specification

### Discovery Operations

#### Peer Advertisement

Only Full nodes and Bridge nodes can advertise their presence:

1. **Tag-based Advertisement**: Full/Bridge nodes advertise under capability tags (e.g., "full", "archival")
2. **DHT Integration**: Uses underlying DHT routing for advertisement
3. **Configurable Interval**: Advertisement timing controlled by `AdvertiseInterval` parameter

#### Peer Discovery

All node types can discover peers:

1. **Tag-based Lookup**: Any node can query for peers advertising under specific tags
2. **Limited Set Management**: Discovered peers are added to bounded peer set
3. **Automatic Updates**: Peer additions/removals trigger update notifications
4. **Connection Tracking**: Monitors peer connectivity state

**Typical Usage:**

- **Light nodes** discover Full and Bridge nodes
- **Full and Bridge nodes** may discover each other

## API Reference

### Discovery Interface

Based on the actual implementation:

```go
// Discovery provides peer discovery and advertisement functionality
type Discovery interface {
    // Advertise announces peer presence under the configured tag (FN/BN only)
    Advertise(ctx context.Context) error

    // Start begins the discovery service and launches the loop that performs
    // peer discovery under specific topic.
    Start(ctx context.Context) error

    // Stop shuts down the discovery service
    Stop(ctx context.Context) error

    // Size returns the current number of peers in the limited set
    Size() int

    // Peers provides a list of discovered peers in the given topic
    // If Discovery hasn't found any peers, it blocks until at least one peer is found
    func Peers(ctx context.Context) ([]peer.ID, error)

    // Discard removes the peer from the peer set and rediscovers more if soft peer limit is not
    // reached. Reports whether peer was removed with bool.
    func Discard(id peer.ID) bool
}
```

### Parameters

```go
type Parameters struct {
    PeersLimit        int           // Maximum peers in limited set
    AdvertiseInterval time.Duration // Interval between advertisements
}

// DefaultParameters returns the default Parameters' configuration values
// for the Discovery module
func DefaultParameters() *Parameters {
    return &Parameters{
        PeersLimit:        5, 
        AdvertiseInterval: time.Hour,
    }
}
```

### Tags

```go
const (
    // fullNodesTag is the tag used to identify full nodes in the discovery service.
    fullNodesTag = "full"
    // archivalNodesTag is the tag used to identify archival nodes in the  discovery service.
    archivalNodesTag = "archival"
)
```

## Implementation Details

### Discovery Service

The Discovery service is the core component that:

1. **Manages Advertisement**: Runs advertisement as background process
2. **Maintains Peer Set**: Uses limited set to bound discovered peers
3. **Provides Notifications**: Calls update callbacks on peer changes
4. **Integrates with DHT**: Uses routing discovery for DHT operations

### Limited Set Management

The Limited Set provides bounded peer management:

- **Size Constraints**: Enforces maximum number of peers via `PeersLimit`
- **Automatic Cleanup**: Removes peers when connections are lost
- **Size Reporting**: Provides current peer count via `Size()` method

### Tag-Based Organization

- Nodes create separate Discovery instances for each tag they manage
- Tags identify node capabilities
- Discovery instances operate independently per tag

## References

1. **Celestia Node**: <https://github.com/celestiaorg/celestia-node>
2. **libp2p Discovery**: <https://docs.libp2p.io/concepts/protocols/#peer-discovery>

---

**Status**: Implemented  
**Version**: 1.0  
**Last Updated**: August 2025
