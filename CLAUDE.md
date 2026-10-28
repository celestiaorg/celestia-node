# CLAUDE.md

Celestia-node implements Celestia's data availability nodes. Node wiring uses `go.uber.org/fx`: `nodebuilder/*/module.go` provides dependencies and `nodebuilder/module.go` composes them per node type.

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

## Testing Infrastructure

- **Swamp** (`nodebuilder/tests/swamp/`): In-process mock networks for integration tests.
- **Tastora** (`nodebuilder/tests/tastora/`): Docker-based E2E tests.
- Integration tests live in `nodebuilder/tests/` and require build tags (see TAGS above).

## Code Conventions

### PR & Commit Style

- PR titles: `type(pkg): [level] subject` (e.g., `fix(service/header): [medium] remove race in core_listener`). CI enforces the format.
- Levels: `[high]` deep review (DAS/availability verification, shrex/p2p protocol, header verification, fraud proofs, state/tx submission, breaking API/config changes, security); `[medium]` standard review for limited-impact code changes; `[low]` docs, tests, or tooling. Pick the higher level when unsure. See [CONTRIBUTING.md#pr-naming](CONTRIBUTING.md#pr-naming).
- Don't repeat the tag in the PR description; the title already carries it.
- Commit messages: conventional commits (e.g., `feat(service/header): Title`)

### Import Ordering

Imports are enforced by `goimports-reviser` with three groups:

1. Standard library
2. Third-party packages
3. `github.com/celestiaorg` packages (company prefix), with `github.com/celestiaorg/celestia-node` as the project

### Proto Changes

Any changes to `*.proto` files require running `make pb-gen` and committing the generated `*.pb.go` files.

### Config Changes

Changes to `nodebuilder/**/config.go` struct fields or `.proto` files are potentially breaking. You can run `make detect-breaking` locally to check.

### Security PRs

- For PRs that resolve Hacken bug bounty reports, do not include details about the bug in the PR description. Instead, link to a Linear issue that contains more details on the bug and the link to the Hacken bug bounty report.
