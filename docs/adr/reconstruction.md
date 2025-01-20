# DRAFT

This document is wip reconstruction protocol descrtipion. It is high level overview of proposal to spark the first iteration of discussion. The document will be split into CIP and ADR later.
## Abstract

This document proposes a new block reconstruction protocol that addresses key bottlenecks in the current implementation, specifically targeting duplicate request reduction and bandwidth optimization. The protocol introduces a structured approach to data retrieval with explicit focus on network resource efficiency and scalability.

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

Engineering time.

Initial draft  aims to be optimised in terms of engineering efforts required for first iteration of implementation. Optimizations marked as optional can be implemented in subsequent updates based on network performance metrics.

## Specification

### Performance Requirements

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

```
1. Connection Establishment
   Client                              Server
      |---- Handshake (node type) ----->|
      |<---- Handshake (node type) -----|

2. Bitmap Subscription
   Client                                   Server
      |---- Subscribe to bitmap ------------->|
      |<---- Initial bitmap ------------------|
      |<---- Updates  ------------------------|
      |<---- End updates(full eds/max samples-|

3. Data Request
   Client                              Server
      |---- Request(bitmap) ----------->|
      |<---- [Samples + Proof] parts ---|
```

### Bitmap Implementation

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


### Core Protocol Components

#### 1. Connection Management

Handshakes, Node Type Identification
- Node type must be declared during connection handshake by each of connection nodes
- Active connection state maintenance on full nodes. List should be maintained by node type
- Connection state should allow subscription for updated

#### 2. Sample Bitmap Protocol

2.1. Bitmap Subscription Flow

- Requested by block height
- Implements one-way stream for bitmap updates
- Full nodes: Stream until complete EDS bitmap
- Light nodes: Stream until max sampling amount reached
- Include end-of-subscription flag for easier debugging

2.2. Update Requirements
- Minimum update frequency: every 10 seconds
- Server-side update triggers:
    - Time-based: Every 5 seconds. Default in base implementation.
    - Optional: Change-based with threshold
- Penalty system for delayed updates

#### 3. Reconstruction State Management

3.1. Global State Components
```go
type RemoteState struct {
    coords [][]peers    // Peer lists by coordinates
    rows []peers        // Peers with row data
    cols []peers        // Peers with column data
    available bitmap    // Available samples bitmap
}

// Basic peer list structure.Structure might be replaced later to implement better 
//peer scoring mechanics 
type peers []peer.ID
```

3.2. State Query Interface

Remote state:
- GetPeersWithSample(Coords) -> []peer.ID
- GetPeersWithAxis(axisIdx, axisType) -> []peer.ID
- Available() -> bitmap

Local state:
- InProgress() -> bitmap. Tracks ongoing fetch sessions to prevent requesting duplicates
- Have() -> bitmap.  Tracks 



#### 4. Data Request
There should be global per block coordinator process that will be responsible for managing the data request process.

4.1. Request Initiation may have multiple strategies:
- Immediate request of all missing samples bitmap receipt
- Optional: Delayed start for bandwidth optimization
    - Wait for X% peer responses
    - Fixed time delay
    - Complete EDS availability
    - Pre-confirmation threshold
    - Combination of conditions

4.1.1 what and were (to request) biggest question.

First iteration of decision engine can be done as simple as possible to allow easier testing of other components and prove the concept. The base properties should be:
- Eliminate requests for duplicate data
- Do not request data, that can be derived from other data. Request just enough data for successful reconstruction


Following potential optimization trategies
- Skip encoded derivable data in response by requesting from peers that have all shares from the same rows/columns
- Optimize proof sizes through range requests
- Optimize proof sizes through subtree proofs, if adjacent subroots are stored
- Parallel request distribution to reduce network load on single peers
- Request from peers with smaller latency


4.2. Sample Request Protocol

4.2.1. Request 

- Update InProgress bitmap before request initiation
- Use shrex for data retrieval. Send bitmap for data request. Shrex has built-in support for requesting rows and samplies, however bitmap-based data retrieval is more general and would be easier to support in future, because it allows requesting multiples pieces of data with single request instead of multiple requests
```go 
type SampleRequest struct {
    Samples *roaring.Bitmap
}
```


4.2.2. Response 
- (base version) Server should respond with samples with proofs

Optiomisations:
- Server may split response into multiple parts
Each part contains packed samples in Range format.
- Each part of shoold have specified message prefix. It would allow client to identify sample container type, which would help to maintain backwards compatibility in case of breaking changes in packing algorithm
- Adjacent samples can be  packed together to share common proofs
- If both Row and column are requested, intersections share can be sent once.


4.3 Client response handling
- If response is timeout, retry request from other peers. Potentially penalize slow peer. 
- Client should verify proofs. If proof is invalid, peer should be penalized
- Verified samples should stored and added to local state `Have`
- Clean up InProgress bitmap
   - Clean up information from remote state to free up memory
- If reconstruction is complete clean up local state, shut down reconstruction process and all subscriptions

### Server-Side Implementation

#### 1. Storage Interface
New storage format needs to be implemented for efficient storage of sampels proofs. The format will be used
initially used for storing ongoing reconstruction process and later can be used for light node storage.

1.1. Core Requirements
- Sample storage with proofs
- Allow purge of proofs on successful reconstruction
- Bitmap query support
- Row/column access implementation
- Accessor interface compliance

1.2. Optional Optimizations
- Bitmap subscription support for callbacks
- Efficient proof generation to reduce proofs size overhead 

#### 2. Request Processing

2.1. Sample Request Handling
- Process bitmap-based requests
- Support multi-part responses
- Implement range-based packaging
- Optimize proof generation

2.2. Response Optimization
- Common proof sharing
- Interval-based response packaging
- Share deduplication for intersections



### Optimization Details

1. Bandwidth Optimization
    - Share common proofs across samples
    - Package adjacent samples
    - Optimize proof sizes for known data
    - Implement efficient encoding schemes

2. Request Distribution
    - Load balancing across peers
    - Geographic optimization
    - Parallel request handling
    - Adaptive timeout management

3. State Management Optimization
    - Efficient bitmap operations
    - Memory-optimized state tracking
    - Progressive cleanup of completed data
    - Optimized proof verification

## Rationale

The design decisions in this proposal are driven by:
1. Need for efficient bandwidth utilization
2. Importance of stable reconstruction under varying conditions
3. Support for network scaling
4. Maintenance of security guarantees



## Backwards Compatibility

In lifespan of protocol it may requires a coordinated network upgrade. Implementation should allow for:
1. Version negotiation
2. Transition period support
3. Fallback mechanisms

## List of core components


1. Handshake protocol
2. Active connections management
3. Bitmap subscription protocol
4. Decision engine and state management
5. Client to request set of samples
6. Samples store 
7. Samples server with packaging



