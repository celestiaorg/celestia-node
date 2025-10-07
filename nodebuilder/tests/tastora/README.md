# Tastora Test Framework

This directory contains the Tastora test framework for Celestia Node integration testing.

## Directory Structure

```
tastora/
├── api/                    # JSON-RPC API contract tests
│   └── blob_test.go       # Blob module JSON-RPC tests
├── integration/           # Go client integration tests  
│   └── blob_test.go       # Blob module Go client tests
├── e2e/                   # End-to-end sanity tests
│   └── e2e_sanity_test.go # Basic functionality validation
├── framework.go           # Core test framework
├── config.go             # Test configuration
└── go.mod                # Module dependencies
```

## Test Categories

### API Tests (`/api`)
- **Purpose**: Test JSON-RPC API contracts and response formats
- **Approach**: Pure JSON-RPC requests with manual HTTP calls
- **Focus**: API compatibility, response structure validation, error handling
- **Example**: `blob_test.go` - Tests all blob APIs via JSON-RPC

### Integration Tests (`/integration`) 
- **Purpose**: Test Go client functionality and business logic
- **Approach**: High-level Go client interfaces with typed method calls
- **Focus**: Feature completeness, error handling, edge cases
- **Example**: `blob_test.go` - Tests blob submission, retrieval, validation

### E2E Tests (`/e2e`)
- **Purpose**: End-to-end sanity testing with minimal setup
- **Approach**: Full workflow validation across multiple modules
- **Focus**: Core functionality, basic flows, system stability
- **Example**: `e2e_sanity_test.go` - Tests basic node operations

## Running Tests

```bash
# Run all tests
go test -v -tags=integration ./...

# Run specific test categories
go test -v -tags=integration ./api/...           # JSON-RPC API tests
go test -v -tags=integration ./integration/...   # Go client tests  
go test -v -tags=integration ./e2e/...           # E2E tests

# Run specific test suites
go test -v -tags=integration -run TestBlobJSONRPCTestSuite
go test -v -tags=integration -run TestBlobTestSuite
go test -v -tags=integration -run TestE2ESanityTestSuite
```

## Adding New Tests

### For JSON-RPC API Tests
1. Create new test file in `/api/` (e.g., `header_test.go`)
2. Follow the pattern in `blob_test.go`
3. Use `makeJSONRPCCall()` for HTTP requests
4. Validate response structure and field types

### For Integration Tests  
1. Create new test file in `/integration/` (e.g., `header_test.go`)
2. Follow the pattern in `blob_test.go`
3. Use Go client interfaces directly
4. Test business logic and error conditions

### For E2E Tests
1. Add new test methods to `e2e_sanity_test.go`
2. Focus on cross-module workflows
3. Keep tests minimal and fast
4. Validate end-to-end user scenarios

## Framework Features

- **Docker-based**: Uses containers for isolated test environments
- **Multi-node**: Supports bridge, light, and full node testing
- **Network simulation**: Creates realistic network topologies
- **Automatic funding**: Funds test accounts automatically
- **Cleanup**: Automatic resource cleanup after tests
- **Logging**: Comprehensive test logging and debugging