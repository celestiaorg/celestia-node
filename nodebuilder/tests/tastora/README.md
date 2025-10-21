# Tastora Testing Framework

Docker-based end-to-end testing framework for Celestia Node modules.

## Quick Start

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

func (s *YourModuleTestSuite) TestYourModule() {
    // Nodes are automatically funded and ready
    bridgeNode := s.framework.GetOrCreateBridgeNode(s.ctx)
    client := s.framework.GetNodeRPCClient(s.ctx, bridgeNode)
    
    // Start testing your module
    // ...
}

func TestYourModuleTestSuite(t *testing.T) {
    suite.Run(t, new(YourModuleTestSuite))
}
```

## Key Features

- **Automatic Node Funding**: All nodes come pre-funded (100 billion utia)
- **Docker-Based**: Real containers for production-like testing
- **Parallel Worker Support**: Built-in support for `--tx.worker.accounts`
- **Robust Cleanup**: Handles Docker race conditions and cleanup failures

## Running Tests

```bash
# Run all Tastora tests
go test -v -tags=integration ./nodebuilder/tests/tastora/

# Run specific test suite
go test -v -tags=integration -run TestTransactionTestSuite ./nodebuilder/tests/tastora/

# Run with timeout
go test -timeout 30m -v -tags=integration ./nodebuilder/tests/tastora/
```

## Configuration

```go
framework := NewFramework(t,
    WithValidators(1),      // Number of validators
    WithBridgeNodes(1),     // Number of bridge nodes
    WithLightNodes(1),      // Number of light nodes
    WithTxWorkerAccounts(8), // Parallel worker accounts
)
```

## Available Test Suites

- **`TestTransactionTestSuite`**: Parallel transaction submission with worker accounts
- **`TestBlobTestSuite`**: Blob module functionality

## Docker Requirements

- Docker Desktop running
- Sufficient resources (4GB RAM, 2 CPUs recommended)
- Network access for container images

## Troubleshooting

### macOS Docker Issues
```bash
# Set correct Docker host for Docker Desktop
export DOCKER_HOST=unix:///Users/$(whoami)/.docker/run/docker.sock
```

### Clean Up
```bash
# Clean up test containers
docker system prune -f
```

## Architecture

- **`framework.go`**: Core testing infrastructure
- **`config.go`**: Configuration options
- **`*_test.go`**: Module-specific test suites

The framework automatically handles:
- Chain initialization and startup
- Node creation and funding
- Container cleanup and error handling
- Network setup and teardown