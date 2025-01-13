# Celestia Node

[![Go Reference](https://pkg.go.dev/badge/github.com/celestiaorg/celestia-node.svg)](https://pkg.go.dev/github.com/celestiaorg/celestia-node)
[![GitHub release (latest by date including pre-releases)](https://img.shields.io/github/v/release/celestiaorg/celestia-node)](https://github.com/celestiaorg/celestia-node/releases/latest)
[![Go CI](https://github.com/celestiaorg/celestia-node/actions/workflows/go-ci.yml/badge.svg)](https://github.com/celestiaorg/celestia-node/actions/workflows/go-ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/celestiaorg/celestia-node)](https://goreportcard.com/report/github.com/celestiaorg/celestia-node)
[![codecov](https://codecov.io/gh/celestiaorg/celestia-node/branch/main/graph/badge.svg?token=CWGA4RLDS9)](https://codecov.io/gh/celestiaorg/celestia-node)

Golang implementation of Celestia's data availability node types (`light` | `full` | `bridge`).

The celestia-node types described above comprise the celestia data availability (DA) network.

The DA network wraps the celestia-core consensus network by listening for blocks from the consensus network and making them digestible for data availability sampling (DAS).

Continue reading [here](https://blog.celestia.org/celestia-mvp-release-data-availability-sampling-light-clients) if you want to learn more about DAS and how it enables secure and scalable access to Celestia chain data.

## Table of Contents

- [Celestia Node](#celestia-node)
  - [Table of Contents](#table-of-contents)
  - [Minimum requirements](#minimum-requirements)
  - [System Requirements](#system-requirements)
  - [Installation](#installation)
  - [API docs](#api-docs)
  - [Node types](#node-types)
  - [Run a node](#run-a-node)
    - [Quick Start with Light Node on arabica](#quick-start-with-light-node-on-arabica)
  - [Environment variables](#environment-variables)
  - [Package-specific documentation](#package-specific-documentation)
  - [Code of Conduct](#code-of-conduct)

## Minimum requirements

| Requirement | Notes          |
| ----------- | -------------- |
| Go version  | 1.23 or higher |

## System Requirements

See the official docs page for system requirements per node type:

- [Bridge](https://docs.celestia.org/nodes/bridge-node#hardware-requirements)
- [Light](https://docs.celestia.org/nodes/light-node#hardware-requirements)
- [Full](https://docs.celestia.org/nodes/full-storage-node#hardware-requirements)

## Installation

```sh
git clone https://github.com/celestiaorg/celestia-node.git
cd celestia-node
make build
sudo make install
```

For more information on setting up a node and the hardware requirements needed, go visit our docs at <https://docs.celestia.org>.

## API docs

The celestia-node public API is documented [here](https://node-rpc-docs.celestia.org/).

## Node types

- **Bridge** nodes - relay blocks from the celestia consensus network to the celestia data availability (DA) network
- **Full** nodes - fully reconstruct and store blocks by sampling the DA network for shares
- **Light** nodes - verify the availability of block data by sampling the DA network for shares

More information can be found [here](https://github.com/celestiaorg/celestia-node/blob/main/docs/adr/adr-003-march2022-testnet.md#legend).

## Run a node

`<node_type>` can be: `bridge`, `full` or `light`.

```sh
celestia <node_type> init
```

```sh
celestia <node_type> start
```

Please refer to [this guide](https://docs.celestia.org/nodes/celestia-node/) for more information on running a node.

### Quick Start with Light Node on arabica

View available commands and their usage:

```sh
make node-help
```

Install celestia node and cel-key binaries:

```sh
make node-install
```

Start a light node with automated setup:

```sh
make light-arabica-up
```

This command:

- Automatically checks wallet balance
- Requests funds from faucet if needed
- Sets node height to latest-1 for quick startup
- Initializes the node if running for the first time

Options:

```sh
make light-arabica-up COMMAND=again    # Reset node state to latest height
make light-arabica-up CORE_IP=<ip>     # Use custom core IP
```

## Environment variables

| Variable                | Explanation                         | Default value | Required |
| ----------------------- | ----------------------------------- | ------------- | -------- |
| `CELESTIA_BOOTSTRAPPER` | Start the node in bootstrapper mode | `false`       | Optional |

## Package-specific documentation

- [Header](./header/doc.go)
- [Share](./share/doc.go)
- [DAS](./das/doc.go)

## Code of Conduct

See our Code of Conduct [here](https://docs.celestia.org/community/coc).
