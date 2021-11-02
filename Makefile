SHELL=/usr/bin/env bash
PROJECTNAME=$(shell basename "$(PWD)")
BUILD_DATE=$(shell date)
LAST_COMMIT=$(shell git rev-parse HEAD)
# TODO (@OrlandoRomo) get version from git tags
CELESTIA_VERSION="0.1.0"

## help: Get more info on make commands.
help: Makefile
	@echo " Choose a command run in "$(PROJECTNAME)":"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
.PHONY: help

## build: Build celestia-node binary.
build:
	@echo "--> Building Celestia"
	@go build -ldflags "-X 'main.buildTime=$(BUILD_DATE)' -X 'main.lastCommit=$(LAST_COMMIT)' -X 'main.semanticVersion=$(CELESTIA_VERSION)'" ./cmd/celestia 
.PHONY: build

## install: Builds and installs the celestia-node binary into the GOBIN directory.
install:
	@echo "--> Installing Celestia"
	@go install ./cmd/celestia
.PHONY: install

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

## test: Running all *_test.go.
test:
	@echo "--> Running tests"
	@go test -v ./...
.PHONY: test

PB_PKGS=$(shell find . -name 'pb' -type d)
PB_CORE=$(shell go list -f {{.Dir}} -m github.com/celestiaorg/celestia-core)
PB_GOGO=$(shell go list -f {{.Dir}} -m github.com/gogo/protobuf)

## pb-gen: Generate protobuf code for all /pb/*.proto files in the project.
pb-gen:
	@echo '--> Generating protobuf'
	@for dir in $(PB_PKGS); \
		do for file in `find $$dir -type f -name "*.proto"`; \
			do protoc -I=. -I=${PB_CORE}/proto/ -I=${PB_GOGO}  --gogofaster_out . $$file; \
			echo '-->' $$file; \
		done; \
	done;
.PHONY: pb-gen
