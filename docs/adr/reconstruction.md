## Abstract

This document proposes a new block reconstruction protocol that addresses key bottlenecks in the current implementation, specifically targeting duplicate request reduction and bandwidth optimization. The protocol introduces a structured approach to data retrieval with an explicit focus on network resource efficiency and scalability.

## Motivation

The current block reconstruction protocol faces several limitations:
1. High frequency of duplicate requests leading to network inefficiency
2. Suboptimal bandwidth utilization
3. Limited scalability with increasing block sizes and node count

Key improvements include:
- Structured bitmap sharing
- Optimized proof packaging
- Efficient state management
- Robust failure handling

This proposal aims to implement an efficient reconstruction protocol that:
- Minimizes duplicate data requests
- Optimizes bandwidth usage through smart data packaging
- Supports network scaling across multiple dimensions
- Maintains stability under varying network conditions

#### Engineering Time

The initial draft aims to be optimized in terms of engineering efforts required for the first iteration of implementation. Optimizations marked as optional can be implemented in subsequent updates based on network performance metrics.

## Specification

### General Performance Requirements

1. Base Scenario Support:
   - 32MB block size
   - Minimum required light nodes for 32MB blocks
   - Network of 50+ full nodes

2. Performance Metrics (in order of priority):
   - System stability
   - Reconstruction throughput (blocks/second)
   - Per-block reconstruction time

3. Scalability Dimensions:
   - Block size scaling
   - Node count scaling

### Protocol Flow 

Diagram below outlines the high-level flow of the proposed protocols. The detailed specifications are provided in the subsequent sections. Full flow diiagrams are available in the end of this document.
```
1. Bitmap Subscription
   Client                                   Server
      |---- Subscribe to bitmap -------------->|
      |<---- Initial bitmap -------------------|
      |<---- Updates  -------------------------|
      |<---- End updates(full eds/max samples)-|

2. Data Request
   Client                              Server
      |---- Request(bitmap) ----------->|
      |<---- [Samples + Proof] parts ---|
```

## Core Components

### 1. Reconstruction Process
There should be a global per-block coordinator process that will be responsible for managing the data request process.

1. Request Initiation may have multiple strategies:
   - Immediate request of all missing samples upon bitmap receipt
   - (Optional): Delayed start for bandwidth optimization
      - Wait for X% peer responses
      - Fixed time delay
      - Complete EDS availability
      - Pre-confirmation threshold
      - Combination of conditions

2. Select which samples to request and from which peers

The first iteration of the decision engine can be implemented as simply as possible to allow easier testing of other components and prove the concept. The base properties should be:
- Eliminate requests for duplicate data
- Do not request data that can be derived from other data. Request just enough data for successful reconstruction

#### First Implementation:
1. Subscribe to bitmap updates
2. Handle bitmap updates. If any sample is not stored and not in progress, request it from a peer
   - Keep track of in-progress requests in local state
3. Handle sample responses
   - Verify proofs. If a proof is invalid, the peer should be penalized
   - Store samples in local store and update local Have state
   - Remove samples from in-progress bitmap
   - Clean up information about sample coordinates from remote state to free up memory
4. If reconstruction is complete, clean up local state and shut down the reconstruction process

#### Potential Optimizations
- Skip encoded derivable data in response by requesting from peers that have all shares from the same rows/columns
- Optimize proof sizes through range requests
- Optimize proof sizes through subtree proofs if adjacent subroots are stored
- Parallel request distribution to reduce network load on single peers
- Request from peers with smaller latency

### 2. State Management
1. Remote State will store information about peers that have samples for given coordinates. If it has full rows/columns, it will be stored in a separate list.
```go
type RemoteState struct {
    coords [][]peers    // Peer lists by coordinates
    rows []peers        // Peers with full row data
    cols []peers        // Peers with full column data
    available bitmap    // Available samples bitmap
}

// Basic peer list structure. Structure might be replaced later to implement better 
// peer scoring mechanics 
type peers []peer.ID
```

Query Interface

Remote state:
```go
func (s *RemoteState) GetPeersWithSample(coords []Coord) []peer.ID
func (s *RemoteState) GetPeersWithAxis(axisIdx int, axisType AxisType) []peer.ID
func (s *RemoteState) Available() bitmap
```

Progress state:
```go
// Tracks ongoing fetch sessions to prevent requesting duplicates
func (s *ProgressState) InProgress() bitmap
// Tracks samples that are already stored locally to notify peers about it
func (s *ProgressState) Have() bitmap
```

## Bitmap Protocol
### Client
- Client should send a request to subscribe to bitmap updates
- If the subscription gets closed or interrupted, client should re-subscribe

#### Request
```protobuf
message SubscribeBitmapRequest {
    uint64 height = 1;
}
```

### Server
- Server implements a one-way stream for bitmap updates
- Server should send the first bitmap update immediately
- Next updates should be sent every 5 seconds
   - (Optional): Server can send updates more frequently if there is a significant change in the bitmap
- Server should send an end-of-subscription flag when no more updates are expected
   - Full nodes: stream until EDS is available on server
   - Light nodes: stream until max sampling amount is reached
      - (Optional): Light node can send a single response with bitmap and end flag upon successful sampling

#### Response
```
message BitmapUpdate {
    Bitmap bitmap = 1;
    bool is_end = 2;
}
```

The protocol utilizes Roaring Bitmaps for efficient bitmap operations and storage. Roaring Bitmaps provide several advantages for the reconstruction protocol:

1. Efficient Operations
   - Fast logical operations (AND, OR, XOR)
   - Optimized for sparse and dense data
   - Memory-efficient storage

2. Implementation Benefits
   - Native support for common bitmap operations
   - Optimized serialization
   - Efficient iteration over set bits
   - Support for rank/select operations

3. Performance Characteristics
   - O(1) for most common operations
   - Compressed storage format
   - Efficient memory utilization
   - Fast bitmap comparisons

The protocol will use 32-bit encoding for bitmaps to have greater multi-language support. Implementation can use one of the encoding-compatible libraries:
- Go: https://github.com/RoaringBitmap/roaring
- Rust: https://github.com/RoaringBitmap/roaring-rs
- C++: https://github.com/RoaringBitmap/CRoaring
- Java: https://github.com/RoaringBitmap/RoaringBitmap

## Samples Request Protocol

### Request

- Use shrex for data retrieval
- Send bitmap for data request. Bitmap should contain coordinates for requested samples
```protobuf 
message SampleRequest {
  height uint64 = 1;
  Bitmap bitmap = 2;
}
```

### Response
Server should respond with samples with proofs defined in shwap CIP [past link].
```protobuf
message SamplesResponse {
  repeated Sample samples = 1;
}
```

#### Optimizations:
- Adjacent samples can have common proofs. Server would need to send shares with common proof in a single response.
  Each part contains packed samples in Range format
- If both Row and column are requested, intersection share can be sent once

## Storage Backend
A new storage format needs to be implemented for efficient storage of sample proofs. The format will be
initially used for storing ongoing reconstruction process and later can be used for light node storage.

1. Core Requirements
- Sample storage with proofs
- Allow purge of proofs on successful reconstruction
- Bitmap query support
- Row/column access implementation
- Accessor interface compliance

2. Optional Optimizations
- Bitmap subscription support for callbacks
- Efficient proof generation to reduce proof size overhead

## Backwards Compatibility

In the lifespan of the protocol, it may require a coordinated network upgrade. Implementation should allow for:
1. Version negotiation
2. Transition period support
3. Fallback mechanisms

## List of Core Components

1. Bitmap subscription protocol
2. Decision engine and state management
3. Client to request set of samples
4. Samples store
5. Samples server with packaging


## Full reconstruction process diagram
```mermaid
sequenceDiagram
    participant N as New Node
    participant RS as Remote State
    participant RP as Reconstruction Processor
    participant PS as Progress State
    participant SS as Samples Store
    participant P1 as Peer 1
    participant P2 as Peer 2
    participant P3 as Peer 3

    Note over N,P3: Phase 1: Bitmap Discovery
    N->>+P1: Subscribe to bitmap updates
    N->>+P2: Subscribe to bitmap updates
    N->>+P3: Subscribe to bitmap updates

    P1-->>-N: Initial bitmap
    P2-->>-N: Initial bitmap
    P3-->>-N: Initial bitmap

    N->>RS: Update remote state
    
    Note over N,P3: Phase 2: Reconstruction Process Start
    N->>RP: Initialize reconstruction
    activate RP

    loop Process Bitmaps
        RP->>PS: Check progress state
        RP->>RS: Query available samples
        
        Note over RP: Select optimal samples & peers
        
        par Request Samples
            RP->>P1: GetSamples(bitmap subset 1)
            RP->>P2: GetSamples(bitmap subset 2)
            RP->>P3: GetSamples(bitmap subset 3)
        end

        PS->>PS: Mark requests as in-progress
    end

    Note over N,P3: Phase 3: Sample Processing
    par Process Responses
        P1-->>RP: Samples + Proofs 1
        P2-->>RP: Samples + Proofs 2
        P3-->>RP: Samples + Proofs 3
    end

    loop For each response
        RP->>SS: Verify & store samples
        RP->>PS: Update progress
        RP->>RS: Update available samples
    end

    Note over N,P3: Phase 4: Completion
    opt Reconstruction Complete
        RP->>SS: Finalize reconstruction
        RP->>PS: Clear progress state
        RP->>RS: Clear remote state
        deactivate RP
    end

    Note over N,P3: Phase 5: Continuous Updates
    loop Until Complete
        P1-->>N: Bitmap updates
        P2-->>N: Bitmap updates
        P3-->>N: Bitmap updates
        N->>RS: Update remote state
    end
```