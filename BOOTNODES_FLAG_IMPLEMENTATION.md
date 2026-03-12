# --p2p.bootnodes CLI Flag Implementation

## Summary
Successfully implemented `--p2p.bootnodes` CLI flag for Celestia node to allow dynamic bootstrap peer configuration at startup. This enables users to specify bootstrap nodes directly in docker-compose commands or CLI without code modifications.

## Changes Made

### 1. Configuration Structure (nodebuilder/p2p/config.go)
- **Added Field**: `BootstrapPeers []string` to Config struct
- **Default**: Empty slice in DefaultConfig()
- **Purpose**: Store CLI-provided bootstrap peer multiaddresses

### 2. Flag Definition (nodebuilder/p2p/flags.go)
- **Flag Name**: `--p2p.bootnodes`
- **Flag Constant**: `bootnodeFlag = "p2p.bootnodes"`
- **Type**: StringSlice (supports multiple values)
- **Help Text**: Documents multiaddr format and usage
- **Validation**: Each bootnode is validated as a valid multiaddr

### 3. Flag Parsing (nodebuilder/p2p/flags.go - ParseFlags function)
- Added multiaddr validation for each bootnode
- Sets Config.BootstrapPeers from parsed CLI values
- Returns error if invalid multiaddr detected

### 4. Bootstrap Logic (nodebuilder/p2p/bootstrap.go)
- **Updated BootstrappersFor()**: Accepts variadic `cliBootnodes ...string` parameter
- **Updated bootstrappersFor()**: CLI bootnodes take precedence over hardcoded list
- **Behavior**: 
  - If CLI bootnodes provided → use them
  - Otherwise → use hardcoded bootstrapList[Network]
- **Updated connectToBootstrappers()**: Passes cfg.BootstrapPeers to BootstrappersFor()

## Tests Added

### Unit Tests (nodebuilder/p2p/flags_test.go)
1. **TestParseFlags_BootnodesParsed**: Verify multiple bootnodes parsed correctly
2. **TestParseFlags_BootnodesInvalidMultiaddr**: Verify invalid multiaddr rejected
3. **TestParseFlags_BootnodesEmpty**: Verify no override when flag not set

### Integration Tests (nodebuilder/p2p/bootstrap_test.go)
1. **TestBootstrappersFor_UsesCliBootnodes**: CLI bootnodes override hardcoded
2. **TestBootstrappersFor_UsesHardcodedWhenNoCli**: Hardcoded used when no CLI
3. **TestBootstrappersFor_EmptyListForPrivateNetwork**: Private network verified empty
4. **TestBootstrappersFor_CliOverridesHardcoded**: CLI overrides hardcoded for mainnet

**Test Results**: ✅ All 27 p2p tests passing

## Usage Examples

### Command Line - Single Bootnode
```bash
celestia bridge start --p2p.bootnodes /ip4/bootstrap1/tcp/2121/p2p/12D3KooWL39tiwtCD1ghouuZGs6dsrKHkSnW3AcJrCWYESkzhYdg
```

### Command Line - Multiple Bootnodes
```bash
celestia bridge start \
  --p2p.bootnodes /ip4/bootstrap1/tcp/2121/p2p/12D3Koo... \
  --p2p.bootnodes /ip4/bootstrap2/tcp/2121/p2p/12D3Koo...
```

### Docker Compose
```yaml
services:
  light1:
    image: celestia-rda:v1.0.0
    command: >
      celestia light start
      --p2p.network private
      --p2p.bootnodes /ip4/bridge1/tcp/2121/p2p/12D3Koo...
      --node.store /root/celestia/light1
```

### CLI Help
```
--p2p.bootnodes strings
  Comma-separated multiaddresses of bootstrap peers.
  Each bootnode helps to bootstrap the node into the network via peer discovery.
  Can be specified multiple times. (Format: multiformats.io/multiaddr)
```

## Verification Steps

Run the test script to verify everything works:
```bash
./test-flag.sh
```

The script checks:
1. ✅ Binary exists at ./build/celestia
2. ✅ Flag appears in help output
3. ✅ Help text contains expected description
4. ✅ ParseFlags unit tests pass
5. ✅ BootstrappersFor integration tests pass

## Architecture Benefits

### Before
- Bootstrap nodes hardcoded per network
- Private network had empty bootstrap list
- No way to configure bootnodes at runtime without editing code

### After
- **Runtime Configuration**: Set bootnodes via CLI flag
- **Network Flexibility**: Same image works for any bootstrap configuration
- **Docker-Compose Friendly**: No container rebuilding needed
- **Override Capability**: CLI values override hardcoded defaults
- **Backward Compatible**: Hardcoded defaults still used when flag not provided

## Code Files Modified

| File | Changes | Lines |
|------|---------|-------|
| nodebuilder/p2p/config.go | Added BootstrapPeers field | 3 |
| nodebuilder/p2p/flags.go | Added flag constant, flag definition, parsing | 35 |
| nodebuilder/p2p/bootstrap.go | Updated BootstrappersFor/bootstrappersFor, connectToBootstrappers | 20 |
| nodebuilder/p2p/flags_test.go | Added 3 new tests | 60 |
| nodebuilder/p2p/bootstrap_test.go | New file with 4 tests | 80 |

## Build & Test Status

```
Build: ✅ PASS
Unit Tests: ✅ 27 tests passing
P2P Package: ✅ All tests passing
Integration: ✅ Flag visible in CLI help
```

## Related Previous Work

This implementation depends on:
- ✅ RDA grid system (implemented previously)
- ✅ DHT discovery with rendezvous (implemented previously)
- ✅ Environment variable support for RDA_EXPECTED_NODES (implemented previously)
- ✅ P2P bootstrap mechanism (existing infrastructure)

The flag complements but is separate from:
- RDA bootstrap peers (via RDABootstrapPeers config field)
- P2P mutual peers (via --p2p.mutual flag)
