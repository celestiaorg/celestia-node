# SHREX Client/Server Specification

## Abstract

This specification defines a request-response protocol that enables nodes to retrieve data from peers. The protocol provides a simple interface for requesting specific data and receiving responses.

## Overview

It implements a client-server architecture for data exchange:

- **SHREX Client**: Makes requests to retrieve data from a specific peer
- **SHREX Server**: Responds to requests by serving data from local storage

## Client

The SHREX client is responsible for:

- Connecting to a specified peer
- Sending a data request
- Receiving and validating the status response
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
     │                     │ Connect            │
     │                     ├───────────────────>│
     │                     │                    │
     │                     │ Send Request       │
     │                     ├───────────────────>│
     │                     │                    │
     │                     │ Receive Status     │
     │                     │<───────────────────┤
     │                     │                    │
     │                     │ Receive Data       │
     │                     │<───────────────────┤
     │                     │                    │
     │ Return Data/Error   │                    │
     │<────────────────────┤                    │
     │                     │                    │
```

## Server

- Accepting incoming connections
- Reading and validating requests
- Retrieving data from local storage
- Sending responses back to clients
- In certain cases (GetNamespaceData), sends non-inclusion proof along with not found

### Request Handling

```text
                Incoming Request
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
            │ Reject   │    │ Get Data    │
            └──────────┘    │ from Store  │
                            └──────┬──────┘
                                   │
                           ┌───────┴────────┐
                           │                │
                          Found          Not Found
                            │                │
                            ▼                ▼
                       ┌──────────┐    ┌──────────┐
                       │ Send OK  │    │ Send NOT │
                       │ + Data   │    │ FOUND    │
                       └──────────┘    └──────────┘
```

## Response Format

All server responses follow a two-phase pattern:

1. **Status Message**: Indicates whether the request can be fulfilled
    - **OK**: Data is available and will follow
    - **READ REQUEST ERROR** - error happened during reading a request from the stream
    - **SEND STATUS ERROR** - error during sending a status back the client
    - **SEND RESPONSE ERROR** - error during sending a status back the client
    - **BAD REQUEST** - request did not pass validation
    - **NOT FOUND**: Requested data is not available
    - **INTERNAL ERROR**: Server encountered an error

2. **Data (if OK)**: The actual requested data(or non inclusion proof in case of GetNamespaceData request)

## Error Handling

### Client Errors

The client reports errors to its caller:

- Connection failures
- Timeouts

### Server Errors

The server handles errors by:

- Sending appropriate status codes
- Rejecting invalid requests

## Resource Protection

The protocol includes mechanisms to protect node resources:

- **Timeouts**: Requests have time limits to prevent resource exhaustion
- **Validation**: Invalid requests are rejected early to save resources

## Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.
