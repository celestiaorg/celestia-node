# Celestia Node gRPC API Design Decisions & Motivations

## Introduction

This document outlines the design decisions and motivations behind the new gRPC-based API for Celestia Node. It represents an initial draft of the API design, with a focus on security, simplicity, and developer experience.

### Document Purpose and Scope

This document serves multiple purposes:

- Explains the reasoning behind key architectural decisions
- Documents the design principles guiding the API development
- Provides context for the protocol buffer definitions
- May serve as a foundation for a future Architecture Decision Record (ADR) in celestia-node

### Important Notes for Readers

#### 1. **Complementary Documentation**

- This document should be read alongside the proto files which contain the detailed API specifications
- The proto definitions include additional technical details and comments that complement this design overview

#### 2. **Current Status**

- This is a first draft and will need further refinement and community feedback
- Some design decisions may evolve as we gather more implementation experience
- Sections marked with "TBD" indicate areas needing further discussion

#### 3. **Implementation Context**

- The design primarily considers the celestia-go-node implementation as it's currently the main reference
- However, the API is designed with language-agnostic principles to support future implementations (e.g., Rust)
- Special attention is paid to security considerations and ease of correct implementation

The following sections detail specific design decisions and their rationales, starting with core design philosophy and moving to service-specific considerations.

[Rest of document follows...]

## Core Design

### 1. API Simplification & User Experience

The API was redesigned with a strong focus on developer experience, while keeping core functionality and security:

- **Security**: Ensuring proper data availability verification and proof checking
- **Separation of Concerns**: Each service handles a specific domain (blobs, shares, proofs) rather than having monolithic endpoints. This makes the API more intuitive and easier to learn.
- **Minimal Surface Area**: The API provides essential operations without trying to solve every edge case in a single RPC call. This approach:
  - Makes the API easier to understand and maintain
  - Encourages composability of basic operations
  - Reduces complexity of individual endpoints

### 2. Protocol Independence

A key design goal was to abstract away protocol-specific details:

- **Removal of DAH Dependencies**: The API minimizes reliance on DataAvailabilityHeader (DAH) to:
  - Reduce coupling to protocol internals
  - Make the API more stable against protocol changes
  - Simplify client implementations
- **Data Root Proofs Focus**: Moving towards data root proofs instead of row roots because:
  - Provides a more direct verification path
  - Reduces complexity of proof verification
  - Better aligns with future protocol optimizations
- **Easier Migration**: Across all services, some requests and responses have V0/V1 suffixes. V0 supports row root-based proofs (matching our current implementation), while V1 uses data root verification. Both options are included to enable a smoother migration to the new API while data root proofs are still under development.

### 3. Data Availability

Data availability verification is fundamental to light node security and must be enforced rigorously through the API. The current implementation has two key limitations:

1. No straightforward mechanism to verify data availability at a given height
2. ShareAvailable API only verifies individual blocks, while proper DAS verification requires checking the entire sampling window preceding the requested height

Proposed Solutions:

1. **Enhanced ShareAvailable Verification**:
  The ShareAvailable call will verify the continuous chain of blocks from the oldest block in the availability window up to the requested height. The call will return false if any block within this range is unavailable.

2. **Integrated Availability Verification**:
  A new `verify_available` option will be added to all data fetching methods (blobs, shares, etc). When enabled:
    - The API will verify availability for both the requested block and all preceding blocks within the sampling window
    - Data will only be returned if the entire chain segment is verified as available
    - Failed availability checks will return a specific `data_not_available` error code

```protobuf
message Options {
    // if verify_available is enabled, node will additionally ensure
    // data availability on top of fetching requested data
    bool verify_available = 1;
    // when proofs_only is enabled, proofs will be returned without sending
    // data.
    bool proofs_only = 2;
}
```

### 4. Error Response Structure

```protobuf
message Status {
    StatusCode code = 1;
    string message = 2;
}
```

Advantages:

- Consistent error reporting
- Rich error information
- Easy to extend
- Supports both technical and user-friendly messages

Some error codes already listed in the draft, however the list might be not full and would need to be revisited with each particular usecase in mind

Standardized status codes

```protobuf
// Common status codes used across all services
enum StatusCode {
  // Reserve 0 for unspecified
  STATUS_UNSPECIFIED = 0;
  // Success codes (1-99)
  STATUS_OK = 1;
  // Error codes (100+)
  // ERROR_DATA_NOT_AVAILABLE is returned if data availability sampling guarantees was required for request
  // and failed. Keep in mind that data availability sampling will be ensured for all blocks in
  // sampling window till the requested height
  ERROR_DATA_NOT_AVAILABLE = 100;
  // Fraud was detected in the chain before requested block
  ERROR_FRAUD_DETECTED = 101;
  ERROR_TIMEOUT = 102;
  // invalid input was submit as part of request. More information is provided in Status message field.
  ERROR_INVALID_REQUEST = 103;
  // unexpected error happened during handling the request
  ERROR_INTERNAL = 104;
  // method is not implemented by the server
  ERROR_NOT_IMPLEMENTED = 105;
}
```

## Service-Specific Decisions

### BlobService

#### 1. Data Submission Approach

Two options were considered for blob submission:

```protobuf
message SubmitDataRequest {
    bytes namespace = 1;
    repeated bytes data = 2;
    SubmitOptions options = 3;
}

message SubmitBlobRequest {
  repeated Blob blobs = 1;
  SubmitOptions options = 2;
}
```

Chose raw data submission over pre-constructed blobs because:

- Simpler for users - they don't need to understand blob internals
- Reduces client-side complexity
- Better separation between data and protocol details

#### 2. Subscription Support

Previously only one subscription endpoint existed to subscribe to all blobs from same namespace.

```protobuf
rpc SubscribeAll(SubscribeAllRequest) returns (stream SubscribeAllResponse);
```

Added new subscription endpoint `SubscribeBlobs`, which allows to subscribe for blobs by commitments:

```protobuf
rpc SubscribeBlobs(BlobsSubscribeRequest) returns (stream BlobsSubscribeResponse);
```

Motivations:

- Essential for async submission workflows.It allows subscribing to blobs submitted in async mode
- Enables event-driven architectures.
- Better for monitoring submitted data
- Reduces polling overhead

### ShareService

Has support for requesting all common sets of shares:

```protobuf
// ShareService - handles data share operations
service ShareService {
  // Core share operations
  rpc GetShare(GetShareRequest) returns (GetShareResponse);
  rpc GetEDS(GetEDSRequest) returns (GetEDSResponse);

  // Range operations - support both V0 (row roots) and V1 (data root) responses
  rpc GetRange(GetRangeRequest) returns (GetRangeResponseV0);
  rpc GetRow(GetRowRequest) returns (GetRowResponseV0);

  // Namespace operations - support both verification approaches
  rpc GetSharesByNamespace(GetSharesByNamespaceRequest) returns (GetSharesByNamespaceResponseV0);

  // Availability check
  rpc SharesAvailable(SharesAvailableRequest) returns (SharesAvailableResponse);
}
```

`SharesAvailable` is reworked. Previously it was ensuring availability of single height. However availability of single height does not give availability guarantees. New version must ensure availability of range of height starting oldest block within availability window to the requested height. `sampling_window` option will allow users to specify smaller range verification.

```protobuf
message SharesAvailableRequest {
  uint64 height = 1;
  optional uint32 sampling_window = 2;  // Blocks to check from height
}

message SharesAvailableResponse {
  celestia.node.v1.common.Status status = 1;
  bool is_available = 2;
  uint64 highest_available_height = 3;
}
```

Benefits:

- More efficient than requesting individual shares
- Better matches common usage patterns
- Reduces number of RPC calls needed
- Enables bulk operations with proofs

### ProofsService

#### 1. Dual Verification Approaches

Across all services some requests and response have V0/V1 suffix. Support both V0 (row roots) and V1 (data root) verification:

```protobuf
service ProofsService {
    rpc VerifySharesV0(VerifySharesRequestV0) returns (VerifyProofResponse);
    rpc VerifyDataProof(VerifyDataRequest) returns (VerifyProofResponse);
}
```

Rationale:

- Enables gradual migration to data root proofs
- Maintains backward compatibility
- Allows clients to choose optimal approach
- Supports different verification needs

#### 2. Proof Type Separation

Clear separation between proof types. Allows explicit distinguishing between inclusion and non-inclusion proofs. Blob commitment proof is identified as well. ProofType also allows proof service know how to verify provided proof:

```protobuf
enum ProofType {
  PROOF_TYPE_DATA_INCLUSION = 1;
  // Contains leaf hashes for the range where namespace would be
  PROOF_TYPE_DATA_NON_INCLUSION = 2;
  // Commitment proof is used for blobs only.
  // It contains first share of the blob and its inclusion proof
  PROOF_TYPE_COMMITMENT = 3;
  // TBD: structure for commitment non-inclusion
  PROOF_TYPE_COMMITMENT_NON_INCLUSION = 4;
}
```

Benefits:

- Clear distinction between use cases
- Better type safety
- Explicit about proof capabilities
- Easier to extend with new proof types

## Future Considerations

### 1. Proof Evolution

The API is designed to evolve with proof systems:

- Easy to add new proof types
- Clean migration paths
- Support for ZK proofs in future
- Flexible verification approaches

### 2. Extensibility

The design enables future extensions:

- New service types can be added. For example for different types of fraud proofs.
- Backward compatibility maintained by following proto guidelines. However, backwards compatibility tests should be implemented for API implementation.
- Clear versioning for API will allow easier migration of clients.

## Implementation Guidelines

### 1. Native Libraries

The API is designed to work with native libraries:

- Core operations in protobuf
- Complex operations in native code
- Clear boundaries between RPC and local processing
- Efficient data handling

### 2. Client Implementation

Recommendations for clients:

- Use native lobs for proof verification when possible
- Implement efficient subscription handling. (resubscription, error handling...)
- Start with support of V0, but keep V1 proofs in mind for future.
- Handle availability checking appropriately

This design creates a foundation for building robust, efficient clients while maintaining flexibility for future protocol evolution.
