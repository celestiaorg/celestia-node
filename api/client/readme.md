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
- `DAAuthToken`: Authentication token for the Bridge node
- `EnableDATLS`: Enable TLS for Bridge node connection

### SubmitConfig
- `DefaultKeyName`: Default key to use for transactions
- `Network`: Network name (e.g., "mocha-4", "private")
- `CoreGRPCConfig`: Configuration for Core node connection

### CoreGRPCConfig
- `Addr`: Address of the Core gRPC server
- `TLSEnabled`: Whether to use TLS for the connection
- `AuthToken`: Authentication token for Core gRPC

## Security
- When using authentication tokens, TLS is strongly recommended
- The client will warn if auth tokens are used without TLS

## Example
See [example.go](https://github.com/celestiaorg/celestia-node/blob/light-lib/api/client/example/example.go) for a complete example of creating a client, submitting a blob, and retrieving it.