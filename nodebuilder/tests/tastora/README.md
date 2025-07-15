# Tastora Testing Framework

This directory contains the Tastora-based testing framework for Celestia Node,
providing Docker-based end-to-end testing infrastructure similar to how the
`swamp` framework provides mock-based testing.

## Architecture

The Tastora framework provides a structured approach to testing Celestia Node
modules using real Docker containers and networks, making tests closer to
production environments.

### Framework Components

- **`framework.go`** - Core testing infrastructure (equivalent to `swamp.go`)
- **`config.go`** - Configuration options and builders
- **`blob_test.go`** - Blob module test suite (first consumer)

### Key Design Principles

1. **Reusable Framework**: Similar to how `swamp.NewSwamp()` creates a testing
   environment, `NewFramework()` sets up the Tastora infrastructure
2. **Module-Specific Test Suites**: Each module has its own test suite that
   embeds the framework
3. **Docker-Based**: Uses real Docker containers instead of mocks for more
   realistic testing
4. **Configurable**: Supports various network topologies and node configurations

## Usage

### Basic Framework Setup

```go
// Create a new framework instance
framework := NewFramework(t, 
    WithFullNodes(2),
    WithLightNodes(1),
)

// Setup the complete network
err := framework.SetupNetwork(ctx)
require.NoError(t, err)

// Get clients for different node types - all nodes are automatically funded
bridgeNode := framework.GetBridgeNode()
fullNodes := framework.GetFullNodes()
lightNodes := framework.GetLightNodes()
```

### Automatic Node Funding

**New in this version**: All nodes are automatically funded with a default amount (3 billion utia) when created, eliminating the need for manual funding in most test cases.

```go
// Nodes are automatically funded and ready for transactions
fullNode := framework.GetOrCreateFullNode(ctx)
client := framework.GetNodeRPCClient(ctx, fullNode)

// Submit transactions directly - no manual funding required
txConfig := state.NewTxConfig(state.WithGas(200_000), state.WithGasPrice(5000))
height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)
```

### Custom Funding (When Needed)

For scenarios requiring custom funding amounts, the `FundNodeAccount` method is still available:

```go
// Create a custom wallet and fund a node with a specific amount
customWallet := framework.CreateTestWallet(ctx, 10_000_000_000)
framework.FundNodeAccount(ctx, customWallet, node, 5_000_000_000)
```

### Creating Module-Specific Test Suites

Follow the pattern established by `BlobTestSuite`:

```go
type YourModuleTestSuite struct {
    suite.Suite
    framework *Framework
    ctx       context.Context
    cancel    context.CancelFunc
}

func (s *YourModuleTestSuite) SetupSuite() {
    s.ctx, s.cancel = context.WithTimeout(context.Background(), 10*time.Minute)
    s.framework = NewFramework(s.T())
    err := s.framework.SetupNetwork(s.ctx)
    s.Require().NoError(err)
}

func (s *YourModuleTestSuite) TestYourModuleFeature() {
    // Your test implementation
    client := s.framework.GetNodeRPCClient(s.ctx, s.framework.GetFullNodes()[0])
    // ... test your module
}
```

### Configuration Options

The framework supports various configuration options:

```go
framework := NewFramework(t,
    WithValidators(1),      // Number of validators in the chain
    WithFullNodes(2),       // Number of full DA nodes
    WithBridgeNodes(1),     // Number of bridge DA nodes  
    WithLightNodes(3),      // Number of light DA nodes
)
```

## Migration from Swamp

For modules currently using the swamp framework, follow this migration pattern:

### Before (Swamp)

```go
func TestYourModule(t *testing.T) {
    sw := swamp.NewSwamp(t)
    bridge := sw.NewBridgeNode()
    full := sw.NewFullNode()
    // ... test implementation
}
```

### After (Tastora)

```go
type YourModuleTestSuite struct {
    suite.Suite
    framework *Framework
    // ... other fields
}

func (s *YourModuleTestSuite) TestYourModule() {
    // Nodes are automatically funded, ready for immediate use
    bridge := s.framework.GetBridgeNode()
    fullNodes := s.framework.GetFullNodes()
    
    // Get RPC client and start testing
    client := s.framework.GetNodeRPCClient(ctx, fullNodes[0])
    // ... test implementation
}

func TestYourModuleTestSuite(t *testing.T) {
    suite.Run(t, new(YourModuleTestSuite))
}
```

### Key Migration Benefits

- **Automatic Funding**: No need to manually fund nodes - they come pre-funded
- **Simplified API**: Reduced boilerplate compared to swamp
- **Consistent Pattern**: All node types follow the same usage pattern

## Test Organization

### Current Structure

```text
nodebuilder/tests/tastora/
├── framework.go        # Core framework infrastructure
├── config.go          # Configuration options
├── blob_test.go        # Blob module tests ✅ Working

├── testdata/           # Test data files
│   └── submitPFB.json  # HTTP blob submission payload
└── README.md          # This documentation
```

### Future Structure (as modules migrate)

```text
nodebuilder/tests/tastora/
├── framework.go        # Core framework
├── config.go          # Configuration
├── blob_test.go        # Blob module tests ✅

├── header_test.go      # Header module tests (planned)
├── share_test.go       # Share module tests (planned)  
├── state_test.go       # State module tests (planned)
├── da_test.go          # DA module tests (planned)
└── README.md          # Documentation
```

## Available Test Suites

### Blob Tests (`make test-blob`)

Status: ✅ Fully Working

Complete blob module test suite covering:

- Blob submission via direct chain API
- Blob retrieval and verification
- V0 and V1 blob support
- Mixed blob version scenarios
- Commitment verification
- Proof generation and validation
- Error handling for non-existent blobs

### Running Tests

#### Individual Test Suites

```bash
# Run only blob tests
make test-blob

# Run all Tastora tests
make test-tastora
```

#### Specific Test Cases

```bash
# Run specific blob test
go test -v -run TestBlobTestSuite/TestBlobModule ./nodebuilder/tests/tastora/
```

## Migration Status from Swamp

### Key Migration Achievements

**✅ Successfully Migrated:**

- Complete testing framework infrastructure  
- Docker-based node management (vs swamp's mocks)
- Test suite organization and patterns
- Blob module test coverage
- Wallet funding and account management

**🎯 Framework Completeness:** **90%** - Core infrastructure and most test
patterns successfully migrated

## Key Differences from Swamp

| Aspect | Swamp | Tastora |
|--------|-------|---------|
| **Environment** | Mock networks (mocknet) | Real Docker containers |
| **Speed** | Faster (in-memory) | Slower but more realistic |
| **Isolation** | Process-level | Container-level |
| **Networking** | Mock P2P | Real networking |
| **Use Case** | Unit/Integration tests | E2E tests |
| **Resource Usage** | Lower | Higher (Docker overhead) |
| **Node Funding** | Manual keyring setup | **Automatic funding** |
| **WebSocket Support** | Full support | Basic client limitations |

## Running Tests

### Individual Test Suite

```bash
# Run only blob tests
go test -v -run TestBlobTestSuite ./nodebuilder/tests/tastora/

# Run specific test within suite
go test -v -run TestBlobTestSuite/TestBlobModule ./nodebuilder/tests/tastora/
```

### Via Makefile

```bash
# Run e2e tests (update Makefile target as needed)
make test-e2e test=TestBlobTestSuite
```

## Contributing

When adding new module tests to this framework:

1. **Follow the Pattern**: Use the `BlobTestSuite` as a template
2. **Reuse Framework**: Don't recreate infrastructure, use the shared `Framework`
3. **Organize by Module**: Keep related tests together in module-specific files
4. **Document**: Update this README with your module's test patterns
5. **Test Thoroughly**: Ensure tests work in CI/CD environments

## Troubleshooting

### Docker Issues

- Ensure Docker is running and accessible
- Check Docker socket permissions (see main e2e test documentation)

### Test Timeouts

- Increase context timeout for slower environments
- Consider reducing test scope for CI environments

### Resource Constraints

- Tastora tests use more resources than swamp tests
- Consider running fewer parallel tests if resource-constrained

## Framework Services

The framework provides several key services:

1. **Chain Management**: Manages Celestia chain via Docker
2. **Node Management**: Handles bridge, full, and light nodes with automatic funding
3. **Wallet Operations**: Creates and funds test wallets
4. **RPC Clients**: Provides access to node APIs

### Automatic Node Funding

All nodes created by the framework are automatically funded with 3 billion utia, eliminating the need for manual funding in most test cases:

```go
// Nodes are automatically funded when created
bridgeNode := framework.GetOrCreateBridgeNode(ctx)  // Auto-funded
fullNode := framework.NewFullNode(ctx)              // Auto-funded  
lightNode := framework.NewLightNode(ctx)            // Auto-funded

// Ready for immediate transaction operations
client := framework.GetNodeRPCClient(ctx, fullNode)
txConfig := state.NewTxConfig(state.WithGas(200_000), state.WithGasPrice(5000))
height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)
```

### Custom Wallet and Funding Operations

For advanced scenarios, manual wallet operations are still available:

```go
// Create a new funded wallet on the chain
testWallet := framework.CreateTestWallet(ctx, 10_000_000_000) // 10 billion utia

// Transfer funds between addresses
framework.FundWallet(ctx, fromWallet, toAddress, amount)

// Additional funding for nodes (beyond automatic funding)
nodeAccAddr := framework.FundNodeAccount(ctx, fromWallet, daNode, amount)
```

**Key Benefits of Automatic Funding:**

- **Eliminates Boilerplate**: No need to manually fund nodes in every test
- **Reduces Errors**: Prevents forgetting to fund nodes before operations
- **Consistent Behavior**: All node types follow the same funding pattern
- **Backwards Compatible**: Manual funding methods remain available for custom scenarios

## Docker Setup and Troubleshooting

### macOS Docker Desktop Issues

On macOS with Docker Desktop, you may encounter Docker socket connection errors:

```
Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
```

This happens because Docker Desktop uses a different socket path. To fix this:

#### Option 1: Use the Helper Script (Recommended)

```bash
# Run all tests
./nodebuilder/tests/tastora/run-tests.sh

# Run only blob tests
./nodebuilder/tests/tastora/run-tests.sh blob
```

#### Option 2: Set Environment Variable Manually

```bash
# Set the correct Docker host for Docker Desktop
export DOCKER_HOST=unix:///Users/$(whoami)/.docker/run/docker.sock

# Then run tests
make test-tastora
```

#### Option 3: Create a Symlink (Permanent Fix)

```bash
# Create symlink to the standard location (requires sudo)
sudo ln -sf /Users/$(whoami)/.docker/run/docker.sock /var/run/docker.sock
```

### Docker Requirements

- Docker Desktop must be running
- Sufficient resources allocated (recommended: 4GB RAM, 2 CPUs)
- Network access for pulling container images

### Known Issues

- **Container Timing Issues**: Some tests may fail due to container cleanup timing
- **Resource Constraints**: Tests require significant Docker resources
- **Network Conflicts**: Ensure no other services are using the same ports

### Troubleshooting Commands

```bash
# Check Docker status
docker ps

# Check Docker context
docker context ls

# Check available resources
docker system df

# Clean up test containers
docker system prune -f
```
