SHELL=/usr/bin/env bash
PROJECTNAME=$(shell basename "$(PWD)")
LDFLAGS="-X 'main.buildTime=$(shell date)' -X 'main.lastCommit=$(shell git rev-parse HEAD)' -X 'main.semanticVersion=$(shell git describe --tags --dirty=-dev)'"

## help: Get more info on make commands.
help: Makefile
	@echo " Choose a command run in "$(PROJECTNAME)":"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
.PHONY: help

## build: Build celestia-node binary.
build:
	@echo "--> Building Celestia"
	@go build -o build/ -ldflags ${LDFLAGS} ./cmd/celestia
.PHONY: build

## clean: Clean up celestia-node binary.
clean:
	@echo "--> Cleaning up ./build"
	@rm -rf build/*

## install: Build and install the celestia-node binary into the GOBIN directory.
install:
	@echo "--> Installing Celestia"
	@go install -ldflags ${LDFLAGS}  ./cmd/celestia
.PHONY: install

## shed: Build cel-shed binary.
cel-shed:
	@echo "--> Building cel-shed"
	@go build ./cmd/cel-shed
.PHONY: cel-shed

## install-shed: Build and install the cel-shed binary into the GOBIN directory.
install-shed:
	@echo "--> Installing cel-shed"
	@go install ./cmd/cel-shed
.PHONY: install-shed

## fmt: Formats only *.go (excluding *.pb.go *pb_test.go). Runs `gofmt & goimports` internally.
fmt:
	@find . -name '*.go' -type f -not -path "*.git*" -not -name '*.pb.go' -not -name '*pb_test.go' | xargs gofmt -w -s
	@find . -name '*.go' -type f -not -path "*.git*"  -not -name '*.pb.go' -not -name '*pb_test.go' | xargs goimports -w -local github.com/celestiaorg
	@go mod tidy
.PHONY: fmt

## lint: Linting *.go files using golangci-lint. Look for .golangci.yml for the list of linters.
lint:
	@echo "--> Running linter"
	@golangci-lint run
.PHONY: lint

## test-unit: Running unit tests
test-unit:
	@echo "--> Running unit tests"
	@go test -v `go list ./... | grep -v node/tests` -covermode=atomic -coverprofile=coverage.out
.PHONY: test-unit

## test-unit-race: Running unit tests with data race detector
test-unit-race:
	@echo "--> Running unit tests with data race detector"
	@go test -v -race `go list ./... | grep -v node/tests`
.PHONY: test-unit-race

## test-swamp: Running swamp tests located in node/tests
test-swamp:
	@echo "--> Running swamp tests"
	@go test -v ./node/tests
.PHONY: test-swamp

## test-swamp: Running swamp tests with data race detector located in node/tests
test-swamp-race:
	@echo "--> Running swamp tests with data race detector"
	@go test -v -race ./node/tests
.PHONY: test-swamp-race

## test-all: Running both unit and swamp tests
test:
	@echo "--> Running all tests without data race detector"
	@go test ./...
	@echo "--> Running all tests with data race detector"
	@go test -race ./...
.PHONY: test

## benchmark: Running all benchmarks
benchmark:
	@echo "--> Running benchmarks"
	@go test -run="none" -bench=. -benchtime=100x -benchmem ./...
.PHONY: benchmark

PB_PKGS=$(shell find . -name 'pb' -type d)
PB_CORE=$(shell go list -f {{.Dir}} -m github.com/tendermint/tendermint)
PB_GOGO=$(shell go list -f {{.Dir}} -m github.com/gogo/protobuf)

## pb-gen: Generate protobuf code for all /pb/*.proto files in the project.
pb-gen:
	@echo '--> Generating protobuf'
	@for dir in $(PB_PKGS); \
		do for file in `find $$dir -type f -name "*.proto"`; \
			do protoc -I=. -I=${PB_CORE}/proto/ -I=${PB_GOGO}  --gogofaster_out=paths=source_relative:. $$file; \
			echo '-->' $$file; \
		done; \
	done;
.PHONY: pb-gen
