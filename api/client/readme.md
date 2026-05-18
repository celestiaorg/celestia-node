# Celestia Client

A Go client library for interacting with the Celestia network, allowing applications to read data from and submit data to Celestia nodes.

## Overview

This client package provides a simplified interface for interacting with Celestia nodes, handling both read operations (via Bridge nodes) and transaction submission (via Core/Consensus nodes).

## Features

- **Read Operations**: Query headers, retrieve blobs, access share data
- **Submit Operations**: Submit blobs to the Celestia network
- **State Management**: Check balances, execute transactions
- **Keyring Integration**: Manage keys for transaction signing
- **Flexible Client Modes**:
  - Read-only mode (Bridge node connection only)
  - Full client mode (Bridge + Core node connections)

## Installation

```bash
go get github.com/celestiaorg/celestia-node/api/client
```

## Usage

### Initializing a Client

```go
// Create a keyring
kr, err := client.KeyringWithNewKey(client.KeyringConfig{
    KeyName:     "my_key",
    BackendName: keyring.BackendTest,
}, "./keys")

// Configure the client
cfg := client.Config{
    ReadConfig: client.ReadConfig{
        BridgeDAAddr: "http://localhost:26658",
        DAAuthToken:  "your_auth_token",
        EnableDATLS:  false,
    },
    SubmitConfig: client.SubmitConfig{
        DefaultKeyName: "my_key",
        Network:        "mocha-4",
        CoreGRPCConfig: client.CoreGRPCConfig{
            Addr:       "celestia-consensus.example.com:9090",
            TLSEnabled: true,
            AuthToken:  "your_core_auth_token",
        },
        // Optional: Enable parallel transaction submission with multiple worker accounts
        // TxWorkerAccounts: 4,
    },
}

// Create a full client
celestiaClient, err := client.New(context.Background(), cfg, kr)
```

### Creating a Read-Only Client

```go
readCfg := client.ReadConfig{
    BridgeDAAddr: "http://localhost:26658",
    DAAuthToken:  "your_auth_token",
    EnableDATLS:  false,
}

readClient, err := client.NewReadClient(context.Background(), readCfg)
```

### Submitting a Blob

```go
namespace := libshare.MustNewV0Namespace([]byte("example"))
blob, err := blob.NewBlob(libshare.ShareVersionZero, namespace, []byte("data"), nil)

height, err := celestiaClient.Blob.Submit(ctx, []*blob.Blob{blob}, nil)
```

**Keyring Security Note:**
If you use the `BackendTest` keyring backend, the client will log a warning about using a plaintext keyring. For production, use the `file` backend for better security.

### Retrieving a Blob

```go
retrievedBlob, err := celestiaClient.Blob.Get(ctx, height, namespace, blob.Commitment)
data := retrievedBlob.Data()
```

### Checking Balance

```go
balance, err := celestiaClient.State.Balance(ctx)
```

## Configuration

### ReadConfig

- `BridgeDAAddr`: Address of the Bridge node
- `DAAuthToken`: Authentication token for the Bridge node. If set, it will be included as a Bearer token in the `Authorization` HTTP header for all bridge node requests.
- `HTTPHeader`: (Optional) Custom HTTP headers to include with each bridge node request. If you manually set an `Authorization` header here while also setting `DAAuthToken`, the client will return an error.
- `EnableDATLS`: Enable TLS for Bridge node connection. **Warning:** If `DAAuthToken` is set and `EnableDATLS` is `false`, the client will log a warning that this setup is insecure.

**Notes:**

- If both `DAAuthToken` and an `Authorization` header are set, the client will return an error to prevent ambiguous authentication.
- Using authentication tokens without TLS is insecure and will trigger a warning in the client logs.

### SubmitConfig

- `DefaultKeyName`: Default key to use for transactions
- `Network`: Network name (e.g., "mocha-4", "private")
- `CoreGRPCConfig`: Configuration for Core node connection
- `TxWorkerAccounts`: (Optional) Number of worker accounts for transaction submission. Default: 0
  - Value of 0 submits transactions immediately (without a submission queue)
  - Value of 1 uses synchronous submission (submission queue with default signer as author of transactions)
  - Value of > 1 uses parallel submission (submission queue with several accounts submitting blobs). Parallel submission is not guaranteed to include blobs in the same order as they were submitted

### CoreGRPCConfig

- `Addr`: Address of the Core gRPC server
- `TLSEnabled`: Whether to use TLS for the connection
- `AuthToken`: Authentication token for Core gRPC

## Security

- When using authentication tokens, TLS is strongly recommended
- The client will warn if auth tokens are used without TLS

## Example

See [example.go](https://github.com/celestiaorg/celestia-node/blob/main/api/client/example/example.go) for a complete example of creating a client, submitting a blob, and retrieving it.
