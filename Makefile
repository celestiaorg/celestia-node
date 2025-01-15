SHELL=/usr/bin/env bash
PROJECTNAME=$(shell basename "$(PWD)")
DIR_FULLPATH=$(shell pwd)
versioningPath := "github.com/celestiaorg/celestia-node/nodebuilder/node"
OS := $(shell uname -s)
LDFLAGS=-ldflags="-X '$(versioningPath).buildTime=$(shell date)' -X '$(versioningPath).lastCommit=$(shell git rev-parse HEAD)' -X '$(versioningPath).semanticVersion=$(shell git describe --tags --dirty=-dev 2>/dev/null || git rev-parse --abbrev-ref HEAD)'"
TAGS=integration
SHORT=
ifeq (${PREFIX},)
	PREFIX := /usr/local
endif
ifeq ($(ENABLE_VERBOSE),true)
	LOG_AND_FILTER = | tee debug.log
	VERBOSE = -v
else
	VERBOSE =
	LOG_AND_FILTER =
endif
ifeq ($(SHORT),true)
	INTEGRATION_RUN_LENGTH = -short
else
	INTEGRATION_RUN_LENGTH =
endif

include celestia-node.mk

## help: Get more info on make commands.
help: Makefile
	@echo " Choose a command run in "$(PROJECTNAME)":"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
.PHONY: help

## install-hooks: Install git-hooks from .githooks directory.
install-hooks:
	@echo "--> Installing git hooks"
	@git config core.hooksPath .githooks
.PHONY: install-hooks

## build: Build celestia-node binary.
build:
	@echo "--> Building Celestia"
	@go build -o build/ ${LDFLAGS} ./cmd/celestia
.PHONY: build

## build-jemalloc: Build celestia-node binary with jemalloc allocator for BadgerDB.
build-jemalloc: jemalloc
	@echo "--> Building Celestia with jemalloc"
	@go build -o build/ ${LDFLAGS} -tags jemalloc ./cmd/celestia
.PHONY: build-jemalloc

## clean: Clean up celestia-node binary.
clean:
	@echo "--> Cleaning up ./build"
	@rm -rf build/*
.PHONY: clean

## cover: Generate code coverage report.
cover:
	@echo "--> Generating Code Coverage"
	@go install github.com/ory/go-acc@latest
	@go-acc -o coverage.txt `go list ./... | grep -v nodebuilder/tests` -- -v
.PHONY: cover

## deps: Install dependencies.
deps:
	@echo "--> Installing Dependencies"
	@go mod download
.PHONY: deps

## install: Install the celestia-node binary.
install:
ifeq ($(OS),Darwin)
	@$(MAKE) go-install
else
	@$(MAKE) install-global
endif
.PHONY: install

## install-global: Install the celestia-node binary (only for systems that support GNU coreutils like Linux).
install-global:
	@echo "--> Installing Celestia"
	@install -v ./build/* -t ${PREFIX}/bin
.PHONY: install-global

## go-install: Build and install the celestia-node binary into the GOBIN directory.
go-install:
	@echo "--> Installing Celestia"
	@go install ${LDFLAGS} ./cmd/celestia
.PHONY: go-install

## cel-shed: Build cel-shed binary.
cel-shed:
	@echo "--> Building cel-shed"
	@go build ./cmd/cel-shed
.PHONY: cel-shed

## install-shed: Build and install the cel-shed binary into the GOBIN directory.
install-shed:
	@echo "--> Installing cel-shed"
	@go install ./cmd/cel-shed
.PHONY: install-shed

## cel-key: Build cel-key binary.
cel-key:
	@echo "--> Building cel-key"
	@go build ./cmd/cel-key
.PHONY: cel-key

## install-key: Build and install the cel-key binary into the GOBIN directory.
install-key:
	@echo "--> Installing cel-key"
	@go install ./cmd/cel-key
.PHONY: install-key

## fmt: Formats only *.go (excluding *.pb.go *pb_test.go).
# Runs `gofmt & goimports` internally.
fmt: sort-imports
	@find . -name '*.go' -type f -not -path "*.git*" -not -name '*.pb.go' -not -name '*pb_test.go' | xargs gofmt -w -s
	@go mod tidy -compat=1.20
	@cfmt -w -m=100 ./...
	@gofumpt -w -extra .
	@markdownlint --fix --quiet --config .markdownlint.yaml .
.PHONY: fmt

## lint: Lint *.go files with golangci-lint and *.md files with markdownlint.
# Look at .golangci.yml for the list of linters.
lint: lint-imports
	@echo "--> Running linter"
	@golangci-lint run
	@markdownlint --config .markdownlint.yaml '**/*.md'
	@cfmt --m=120 ./...
.PHONY: lint

## test-unit: Run unit tests.
test-unit:
	@echo "--> Running unit tests"
	@go test $(VERBOSE) -covermode=atomic -coverprofile=coverage.txt `go list ./... | grep -v nodebuilder/tests` $(LOG_AND_FILTER)
.PHONY: test-unit

## test-unit-race: Run unit tests with data race detector.
test-unit-race:
	@echo "--> Running unit tests with data race detector"
	@go test $(VERBOSE) -race -covermode=atomic -coverprofile=coverage.txt `go list ./... | grep -v nodebuilder/tests` $(LOG_AND_FILTER)
.PHONY: test-unit-race

## test-integration: Run integration tests located in nodebuilder/tests.
test-integration:
	@echo "--> Running integrations tests $(VERBOSE) -tags=$(TAGS) $(INTEGRATION_RUN_LENGTH)"
	@go test $(VERBOSE) -timeout=20m -tags=$(TAGS) $(INTEGRATION_RUN_LENGTH) ./nodebuilder/tests
.PHONY: test-integration

## test-integration-race: Run integration tests with data race detector located in nodebuilder/tests.
test-integration-race:
	@echo "--> Running integration tests with data race detector -tags=$(TAGS)"
	@go test -race -tags=$(TAGS) ./nodebuilder/tests
.PHONY: test-integration-race

## benchmark: Run all benchmarks.
benchmark:
	@echo "--> Running benchmarks"
	@go test -run="none" -bench=. -benchtime=100x -benchmem ./...
.PHONY: benchmark

PB_PKGS=$(shell find . -name 'pb' -type d)
PB_CORE=$(shell go list -f {{.Dir}} -m github.com/tendermint/tendermint)
PB_GOGO=$(shell go list -f {{.Dir}} -m github.com/gogo/protobuf)
PB_CELESTIA_APP=$(shell go list -f {{.Dir}} -m github.com/celestiaorg/celestia-app)
PB_NMT=$(shell go list -f {{.Dir}} -m github.com/celestiaorg/nmt)
PB_NODE=$(shell pwd)

## pb-gen: Generate protobuf code for all /pb/*.proto files in the project.
pb-gen:
	@echo '--> Generating protobuf'
	@for dir in $(PB_PKGS); \
		do for file in `find $$dir -type f -name "*.proto"`; \
			do protoc -I=. -I=${PB_CORE}/proto/ -I=${PB_NODE} -I=${PB_GOGO} -I=${PB_CELESTIA_APP}/proto -I=${PB_NMT} --gogofaster_out=paths=source_relative:. $$file; \
			echo '-->' $$file; \
		done; \
	done;
.PHONY: pb-gen

## openrpc-gen: Generate OpenRPC spec for celestia-node's RPC API.
openrpc-gen:
	@go run ${LDFLAGS} ./cmd/celestia docgen
.PHONY: openrpc-gen

## lint-imports: Lint only Go imports.
lint-imports:
# flag -set-exit-status doesn't exit with code 1 as it should, so we use find until it is fixed by goimports-reviser
	@echo "--> Running imports linter"
	@for file in `find . -type f -name '*.go'`; \
		do goimports-reviser -list-diff -set-exit-status -company-prefixes "github.com/celestiaorg" -project-name "github.com/celestiaorg/celestia-node" -output stdout $$file \
		 || exit 1;  \
    done;
.PHONY: lint-imports

## sort-imports: Sort Go imports.
sort-imports:
	@goimports-reviser -company-prefixes "github.com/celestiaorg" -project-name "github.com/celestiaorg/celestia-node" -output stdout ./...
.PHONY: sort-imports

## adr-gen: Generate ADR from template. Must set NUM and TITLE parameters.
adr-gen:
	@echo "--> Generating ADR"
	@curl -sSL https://raw.githubusercontent.com/celestiaorg/.github/main/adr-template.md > docs/architecture/adr-$(NUM)-$(TITLE).md
.PHONY: adr-gen

## telemetry-infra-up: Launch local telemetry infra (grafana, jaeger, loki, pyroscope, and otel-collector).
# You can access the grafana instance at localhost:3000 and login with admin:admin.
telemetry-infra-up:
	PWD="${DIR_FULLPATH}/docker/telemetry" docker-compose -f ./docker/telemetry/docker-compose.yml up
.PHONY: telemetry-infra-up

## telemetry-infra-down: Tears the telemetry infra down. Persists the stores for grafana, prometheus, and loki.
telemetry-infra-down:
	PWD="${DIR_FULLPATH}/docker/telemetry" docker-compose -f ./docker/telemetry/docker-compose.yml down
.PHONY: telemetry-infra-down

## goreleaser: List Goreleaser commands and checks if GoReleaser is installed.
goreleaser: Makefile
	@echo " Choose a goreleaser command to run:"
	@sed -n 's/^## goreleaser/goreleaser/p' $< | column -t -s ':' |  sed -e 's/^/ /'
	@goreleaser --version
.PHONY: goreleaser

## goreleaser-build: Builds the celestia binary using GoReleaser for your local OS.
goreleaser-build:
	goreleaser build --snapshot --clean --single-target
.PHONY: goreleaser-build

## goreleaser-release: Builds the release celestia binaries as defined in .goreleaser.yaml.
# This requires there be a git tag for the release in the local git history.
goreleaser-release:
	goreleaser release --clean --fail-fast --skip-publish
.PHONY: goreleaser-release

# detect changed files and parse output
# to inspect changes to nodebuilder/**/config.go fields
CHANGED_FILES      = $(shell git diff --name-only origin/main...HEAD)
detect-breaking:
	@BREAK=false
	@for file in ${CHANGED_FILES}; do \
		if echo $$file | grep -qE '\.proto$$'; then \
			BREAK=true; \
		fi; \
		if echo $$file | grep -qE 'nodebuilder/.*/config\.go'; then \
			DIFF_OUTPUT=$$(git diff origin/main...HEAD $$file); \
			if echo "$$DIFF_OUTPUT" | grep -qE 'type Config struct|^\s+\w+\s+Config'; then \
				BREAK=true; \
			fi; \
		fi; \
	done; \
	if [ "$$BREAK" = true ]; then \
		echo "break detected"; \
		exit 1; \
	else \
		echo "no break detected"; \
		exit 0; \
	fi
.PHONY: detect-breaking


# Copied from https://github.com/dgraph-io/badger/blob/main/Makefile
USER_ID      = $(shell id -u)
HAS_JEMALLOC = $(shell test -f /usr/local/lib/libjemalloc.a && echo "jemalloc")
JEMALLOC_URL = "https://github.com/jemalloc/jemalloc/releases/download/5.2.1/jemalloc-5.2.1.tar.bz2"

## jemalloc: Install jemalloc allocator.
jemalloc:
	@if [ -z "$(HAS_JEMALLOC)" ] ; then \
		mkdir -p /tmp/jemalloc-temp && cd /tmp/jemalloc-temp ; \
		echo "Downloading jemalloc..." ; \
		curl -s -L ${JEMALLOC_URL} -o jemalloc.tar.bz2 ; \
		tar xjf ./jemalloc.tar.bz2 ; \
		cd jemalloc-5.2.1 ; \
		./configure --with-jemalloc-prefix='je_' --with-malloc-conf='background_thread:true,metadata_thp:auto'; \
		make ; \
		if [ "$(USER_ID)" -eq "0" ]; then \
			make install ; \
		else \
			echo "==== Need sudo access to install jemalloc" ; \
			sudo make install ; \
		fi ; \
		cd /tmp ; \
		rm -rf /tmp/jemalloc-temp ; \
	fi
.PHONY: jemalloc
