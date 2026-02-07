# API Cross-Version Compatibility Tests

Bidirectional cross-version compatibility tests that detect breaking API changes.

## How It Works

1. **Current client → Old server**: Uses current codebase's RPC client to connect to an old server version
2. **Old client → Current server**: Compiles client code from an old version inside Docker and connects to current server

Both directions must pass to ensure API compatibility.

## Running Tests

```bash
go test -tags=integration -v ./nodebuilder/tests/tastora/api/
```

## What Gets Tested

- **Node API**: Info, Ready
- **Header API**: LocalHead, GetByHeight, GetByHash, SyncState, NetworkHead, Tail
- **State API**: AccountAddress, Balance, BalanceForAddress
- **P2P API**: Info, Peers, NATStatus, BandwidthStats, ResourceState, PubSubTopics
- **Share API**: SharesAvailable, GetShare, GetSamples, GetEDS, GetRow, GetNamespaceData, GetRange
- **DAS API**: SamplingStats, WaitCatchUp
- **Blob API**: Submit, Get, GetAll, GetProof, Included, GetCommitmentProof
