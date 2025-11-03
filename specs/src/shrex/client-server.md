# SHREX Client/Server Specification

## Abstract

This specification defines a request-response protocol that enables nodes to retrieve data from peers. The protocol provides a simple interface for requesting specific data and receiving responses.

## Overview

It implements a client-server architecture for data exchange:

- **SHREX Client**: Makes requests to retrieve data from a specific peer
- **SHREX Server**: Responds to requests by serving data from local storage

## Transport Protocol

It uses libp2p streaming as its transport layer.

### Protocol ID

The protocol ID follows the format:

```text
/<network-id>/shrex/v0.1.0/<request-name>
```

Where:

- `<network-id>`: Network identifier
- `/shrex/v0.1.0/`: Protocol name and version
- `<request-name>`: Type of request being made (e.g., "row", "sample", "eds")

### Stream Lifecycle

1. **Stream Opening**: Client opens a new stream to the server using the protocol ID
2. **Request Phase**: Client writes request and closes the write side
3. **Response Phase**: Server sends status message
   - If status is `OK`: Server sends data payload, then closes stream
   - If status is not `OK`: Server closes stream immediately
4. **Stream Closing**: Stream is closed after response is complete

### Timeouts

Both client and server implement timeouts to protect resources:

- **Read Timeout**: Maximum time to read data from stream
- **Write Timeout**: Maximum time to write data to stream
- **Handle Request Timeout**: Maximum time for server to process request

## Message Format

### Encoding

Request and response messages use different encoding schemes:

- **Request Messages**: Binary encoded using `MarshalBinary()` method
- **Response Messages**: Protocol Buffers (protobuf) encoded

### Request Messages

Request messages are binary encoded.

The request is serialized to binary format and written directly to the stream. Each request type includes:

- Height: Block height for the requested data
- Type-specific parameters

### Response Messages

#### Status Message

Every server response begins with a status message:

```protobuf
enum Status {
   INVALID = 0;      // Invalid/unknown status
   OK = 1;           // Data found and will be sent
   NOT_FOUND = 2;    // Requested data not found
   INTERNAL = 3;     // Internal server error
}

message Response {
   Status status = 1;
}
```

#### Data Payload

Data is only sent when status is `OK`. After sending the OK status, the server streams the requested data and then closes the stream.

For all other status codes (`NOT_FOUND`, `INTERNAL`), the server closes the stream immediately after sending the status message without sending any data.

The format and encoding of the data payload is defined in the SHWAP specification.

For certain request types (e.g., GetNamespaceData), a non-inclusion proof may be sent instead of data when the namespace is not present.

## Supported Endpoints

SHREX supports multiple endpoint types for retrieving different kinds of data. All endpoints follow the same request-response pattern but return different data structures.

### Common Types

```protobuf
message Share {
   bytes data = 1;
}

enum AxisType {
   ROW = 0;
   COL = 1;
}
```

### Row Endpoint

Retrieves a row of shares (either left or right half).

**Response Message:**

```protobuf
message Row {
   repeated Share shares_half = 1;
   HalfSide half_side = 2;

   enum HalfSide {
      LEFT = 0;
      RIGHT = 1;
   }
}
```

**Fields:**

- `shares_half`: The shares from the requested half of the row
- `half_side`: Indicates which half (left or right) is being returned

### Sample Endpoint

Retrieves a single share with its proof.

**Response Message:**

```protobuf
message Sample {
   Share share = 1;
   proof.pb.Proof proof = 2;
   AxisType proof_type = 3;
}
```

**Fields:**

- `share`: The requested share data
- `proof`: Merkle proof for the share
- `proof_type`: Indicates if proof is for row or column axis

### NamespaceData Endpoint

Retrieves all shares for a specific namespace across all rows.

**Response Message:**

```protobuf
message NamespaceData {
   repeated RowNamespaceData namespaceData = 1;
}

message RowNamespaceData {
   repeated Share shares = 1;
   proof.pb.Proof proof = 2;
}
```

**Fields:**

- `namespaceData`: Collection of namespace data from each row
- `shares`: Shares belonging to the namespace in a specific row
- `proof`: Proof for the namespace data (may be inclusion or non-inclusion proof)

### RangeNamespaceData Endpoint

Retrieves namespace data for a range of rows.

**Response Message:**

```protobuf
message RangeNamespaceData {
   repeated RowShares shares = 1;
   proof.pb.Proof firstIncompleteRowProof = 2;
   proof.pb.Proof lastIncompleteRowProof = 3;
}

message RowShares {
   repeated Share shares = 1;
}
```

**Fields:**

- `shares`: Collection of shares from each row in the range
- `firstIncompleteRowProof`: Proof for the first incomplete row (if applicable)
- `lastIncompleteRowProof`: Proof for the last incomplete row (if applicable)

### EDS Endpoint

Retrieves the complete Extended Data Square (EDS) for a block.

**Response:**

- Returns `rsmt2d.ExtendedDataSquare` - the full erasure-coded data square
- The EDS contains all shares organized in a 2D matrix structure
- Includes both original data and erasure-coded redundancy shares

**Note:** This endpoint returns the entire data square and may transfer large amounts of data. The exact encoding format follows the rsmt2d specification.

### Protocol ID Format

Each endpoint has its own protocol ID:

```text
/<network-id>/shrex/v0.1.0/row_v0
/<network-id>/shrex/v0.1.0/sample_v0
/<network-id>/shrex/v0.1.0/nd_v0
/<network-id>/shrex/v0.1.0/rangeNamespaceData_v0
/<network-id>/shrex/v0.1.0/eds_v0
```

## Client

The SHREX client is responsible for:

- Connecting to a specified peer
- Sending a data request
- Receiving and validating the status response
- Reading the data payload (if status is OK)
- Returning the result to the caller

### Input and Output

**Input**:

- Target peer ID
- Request parameters (height, type-specific data)

**Output**:

- Retrieved data (on success)
- Error information (on failure)

### Request Flow

```text
┌──────────┐         ┌────────────┐         ┌────────┐
│  Getter  │         │   Client   │         │ Server │
└────┬─────┘         └─────┬──────┘         └───┬────┘
     │                     │                    │
     │ Request(peer, data) │                    │
     ├────────────────────>│                    │
     │                     │                    │
     │                     │ Open Stream        │
     │                     ├───────────────────>│
     │                     │                    │
     │                     │ Send Request       │
     │                     ├───────────────────>│
     │                     │                    │
     │                     │ Close Write        │
     │                     ├───────────────────>│
     │                     │                    │
     │                     │ Receive Status     │
     │                     │<───────────────────┤
     │                     │                    │
     │                     │ Receive Data       │
     │                     │ (if status OK)     │
     │                     │<───────────────────┤
     │                     │                    │
     │ Return Data/Error   │                    │
     │<────────────────────┤                    │
     │                     │                    │
```

### Error Handling

The client maps status codes to errors:

| Status Code | Error | Description |
|------------|-------|-------------|
| `OK` | None | Success, data received |
| `NOT_FOUND` | `ErrNotFound` | Requested data not available |
| `INTERNAL` | `ErrInternalServer` | Server encountered an error |
| `INVALID` or unknown | `ErrInvalidResponse` | Invalid or unexpected status |

Additional client-side errors:

- **Connection Errors**: Stream opening failures
- **Timeout Errors**: Request exceeded time limits
- **Rate Limit Errors**: Indicated by EOF when reading status (server closed stream without response)

## Server

The SHREX server is responsible for:

- Accepting incoming connections
- Reading and validating requests
- Retrieving data from local storage
- Sending status responses
- Sending data payloads (when applicable)
- Sending non-inclusion proofs (for certain request types when data not found)

### Request Handling

```text
                Incoming Stream
                       │
                       ▼
                 ┌──────────────┐
                 │ Read Request │
                 └──────┬───────┘
                        │
                ┌───────┴────────┐
                │                │
             Error            Success
                │                │
                ▼                ▼
         ┌───────────┐    ┌─────────────┐
         │ Reset     │    │ Close Read  │
         │ Stream    │    │             │
         └───────────┘    └──────┬──────┘
                                 │
                                 ▼
                          ┌──────────────┐
                          │ Validate     │
                          │ Request      │
                          └──────┬───────┘
                                 │
                         ┌───────┴────────┐
                         │                │
                      Invalid          Valid
                         │                │
                         ▼                ▼
                    ┌──────────┐    ┌─────────────┐
                    │ Reset    │    │ Get Data    │
                    │ Stream   │    │ from Store  │
                    └──────────┘    └──────┬──────┘
                                           │
                                           ▼
                                    ┌──────────────┐
                                    │ Set Write    │
                                    │ Deadline     │
                                    └──────┬───────┘
                                           │
                                   ┌───────┴────────┐
                                   │                │
                                 Found          Not Found
                                   │                │
                                   ▼                ▼
                              ┌──────────┐    ┌──────────┐
                              │ Send OK  │    │ Send NOT │
                              │ Status   │    │ FOUND    │
                              └────┬─────┘    │ Status   │
                                   │          └──────────┘
                                   ▼
                              ┌──────────┐
                              │ Send     │
                              │ Data     │
                              └──────────┘
```

### Status Response Behavior

The server sends status codes based on the following conditions:

| Status Code | Condition | Stream Behavior |
|------------|-----------|-----------------|
| `OK` | Data found and ready | Send status, then send data |
| `NOT_FOUND` | Data not in store | Send status, close stream |
| `INTERNAL` | Error retrieving data | Send status, close stream |
| N/A (no status sent) | Invalid request | Reset stream without response |
| N/A (no status sent) | Error reading request | Reset stream without response |

## Resource Protection

The protocol includes mechanisms to protect node resources:

- **Timeouts**: Requests have time limits to prevent resource exhaustion
- **Validation**: Invalid requests are rejected early to save resources

## Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.
