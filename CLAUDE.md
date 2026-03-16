# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Celestia-node is the Go implementation of Celestia's data availability (DA) node. It implements two node types:

- **Bridge**: Bridges the Celestia consensus network to the DA network by connecting to a celestia-core node, listening for blocks, and broadcasting ExtendedHeaders
- **Light**: Lightweight node that verifies data availability by sampling 16 random shares per block

## Common Commands

### Build & Install

```bash
make build              # Build celestia binary to build/
make install            # Install celestia binary
make cel-shed           # Build cel-shed utility
make cel-key            # Build cel-key utility
```

### Testing

```bash
make test-unit                           # Run unit tests with coverage
make test-unit-race                      # Run unit tests with race detector
go test ./path/to/package/...            # Run tests for a specific package
go test -run TestName ./path/to/pkg/...  # Run a single test

# Integration tests (in nodebuilder/tests/, require build tags)
make test-integration TAGS=blob          # Run blob integration tests
make test-integration TAGS=p2p           # Run p2p integration tests
make test-integration SHORT=true TAGS=da # Run with -short flag

# Available integration TAGS: api, blob, da, fraud, nd, p2p, reconstruction, sync, share, pruning

# Tastora E2E tests (Docker-based)
make test-blob                           # Blob module E2E tests
make test-e2e-sanity                     # E2E sanity suite
make test-tastora                        # All Tastora tests
```

### Code Quality

```bash
make lint               # Run golangci-lint + markdownlint + cfmt
make fmt                # Format code (gofmt, goimports, cfmt, gofumpt, markdownlint)
make sort-imports       # Sort Go imports
make lint-imports       # Lint Go import ordering
```

### Code Generation

```bash
make pb-gen             # Regenerate protobuf files (requires protoc)
make openrpc-gen        # Generate OpenRPC spec
go generate ./...       # Regenerate mocks (mockgen)
```

## Architecture

### Dependency Injection

The project uses **go.uber.org/fx** for dependency injection. Each module in `nodebuilder/` has a `module.go` that provides its fx dependencies. The `nodebuilder/module.go` composes all modules together, with different configurations per node type.

### nodebuilder/ — Node Construction

Each subdirectory is a module providing a specific capability:

- `blob/` — Blob submission and retrieval
- `core/` — Connection to celestia-core consensus node
- `da/` — Data availability interface
- `das/` — Data availability sampling orchestration
- `fraud/` — Fraud proof handling
- `header/` — Header syncing, storage, and exchange
- `p2p/` — libp2p networking, pubsub, peer management
- `share/` — Share retrieval and availability verification
- `state/` — On-chain state, transaction submission, fee estimation
- `rpc/` — JSON-RPC API server (via go-jsonrpc)
- `pruner/` — Data pruning
- `blobstream/` — Blobstream proof generation

Each module typically has: `config.go` (configuration), `module.go` (fx providers), interface definitions, and `mocks/` directory with mockgen-generated mocks.

### Core Domain Packages (top-level)

- `header/` — ExtendedHeader type (block header + DAH + validator set + commit), syncer, store, P2P exchange
- `share/` — Share types, namespace retrieval, availability interface with light/full implementations
- `share/shwap/` — Share exchange protocol (Shwap) with sub-protocols: shrex (share exchange over libp2p streams), bitswap
- `das/` — DAS coordinator and workers that sample shares to verify availability
- `blob/` — Blob parsing, commitment proofs, service layer
- `core/` — Listener, fetcher, exchange for interacting with celestia-core
- `state/` — Core access to chain state, transaction building
- `store/` — EDS (Extended Data Square) persistent storage

### Key Interfaces

- `share.Availability` — Verifies data availability (light vs full implementations)
- `share/shwap.Getter` — Retrieves shares/EDS/namespace data from the network
- Each `nodebuilder/*/` module defines a `Module` interface that is the RPC API surface

### Networking

Built on **libp2p** with:

- PubSub (gossipsub) for header propagation (HeaderSub) and share notifications (ShrexSub)
- Streams for direct share exchange (Shrex)
- Bitswap for share retrieval
- Kademlia DHT for peer discovery

### Testing Infrastructure

- **Swamp** (`nodebuilder/tests/swamp/`): In-process mock network for integration tests. Creates mock networks with bridge/full/light nodes for testing inter-node communication.
- **Tastora** (`nodebuilder/tests/tastora/`): Docker-based E2E framework using real containers.
- Integration tests use build tags (see TAGS above) and live in `nodebuilder/tests/`.

## Code Conventions

### PR & Commit Style

- PR titles: `pkg: Concise title` (e.g., `service/header: Remove race in core_listener`)
- Commit messages: conventional commits recommended (e.g., `feat(service/header): Title`)

### Import Ordering

Imports are enforced by `goimports-reviser` with three groups:

1. Standard library
2. Third-party packages
3. `github.com/celestiaorg` packages (company prefix), with `github.com/celestiaorg/celestia-node` as the project

### Proto Changes

Any changes to `*.proto` files require running `make pb-gen` and committing the generated `*.pb.go` files.

### Config Changes

Changes to `nodebuilder/**/config.go` struct fields or `.proto` files are potentially breaking. You can run `make detect-breaking` locally to check.
