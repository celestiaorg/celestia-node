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

Requests use shwap id and associated encoding. Each request type includes:
- Height: Block height for the requested data
- Type-specific parameters

### Response Messages

#### Status Message

After handling requests server MUST send Status message indicating result of handling

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

The format and encoding of the data payload is defined in the [SHWAP](https://github.com/celestiaorg/CIPs/blob/main/cips/cip-019.md) specification.

For certain request types (e.g., GetNamespaceData), a non-inclusion proof may be sent instead of data when the namespace is not present.

## Supported Endpoints

SHREX supports multiple endpoint types for retrieving different kinds of data. All endpoints follow the same request-response pattern but return different data structures.

All ProtocolIDs MUST be prefixed with the `networkID`.

| ProtocolID                                       | Request                                                                                                                                                           | Response                                                                                                                                                     |
|--------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `<networkID>/shrex/v0.1.0/row_v0`                | [RowID](https://github.com/celestiaorg/CIPs/blob/main/cips/cip-019.md#rowid)                                                                                      | [Row](https://github.com/celestiaorg/CIPs/blob/main/cips/cip-019.md#row-container)                                                                           |
| `<networkID>/shrex/v0.1.0/sample_v0`             | [SampleID](https://github.com/celestiaorg/CIPs/blob/main/cips/cip-019.md#sampleid)                                                                                | [Sample](https://github.com/celestiaorg/CIPs/blob/main/cips/cip-019.md#sample-container)                                                                     |
| `<networkID>/shrex/v0.1.0/nd_v0`                 | [NamespaceDataID](https://github.com/celestiaorg/celestia-node/blob/2c991a8cbb3be9afd413472e127b0e2b6e770100/share/shwap/namespace_data_id.go#L21C1-L26C2)        | [NamespaceData](https://github.com/celestiaorg/celestia-node/blob/2c991a8cbb3be9afd413472e127b0e2b6e770100/share/shwap/namespace_data.go#L20)                |
| `<networkID>/shrex/v0.1.0/eds_v0`                | [EdsID](https://github.com/celestiaorg/CIPs/blob/main/cips/cip-019.md#edsid)                                                                                      | [Eds](https://github.com/celestiaorg/CIPs/blob/main/cips/cip-019.md#eds-container)                                                                           |
| `<networkID>/shrex/v0.1.0/rangeNamespaceData_v0` | [RangeNamespaceDataID](https://github.com/celestiaorg/celestia-node/blob/2c991a8cbb3be9afd413472e127b0e2b6e770100/share/shwap/range_namespace_data_id.go#L31-L37) | [RangeNamespaceData](https://github.com/celestiaorg/celestia-node/blob/2c991a8cbb3be9afd413472e127b0e2b6e770100/share/shwap/range_namespace_data.go#L38-L42) |

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
