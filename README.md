# Celestia Node

[![Go Reference](https://pkg.go.dev/badge/github.com/celestiaorg/celestia-node.svg)](https://pkg.go.dev/github.com/celestiaorg/celestia-node)
[![GitHub release (latest by date including pre-releases)](https://img.shields.io/github/v/release/celestiaorg/celestia-node)](https://github.com/celestiaorg/celestia-node/releases/latest)
[![test](https://github.com/celestiaorg/celestia-node/actions/workflows/test.yml/badge.svg)](https://github.com/celestiaorg/celestia-node/actions/workflows/test.yml)
[![lint](https://github.com/celestiaorg/celestia-node/actions/workflows/lint.yml/badge.svg)](https://github.com/celestiaorg/celestia-node/actions/workflows/lint.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/celestiaorg/celestia-node)](https://goreportcard.com/report/github.com/celestiaorg/celestia-node)
[![codecov](https://codecov.io/gh/celestiaorg/celestia-node/branch/main/graph/badge.svg?token=CWGA4RLDS9)](https://codecov.io/gh/celestiaorg/celestia-node)

Golang implementation of Celestia's data availability node types (`light` | `full` | `bridge`).

The celestia-node types described above comprise the celestia data availability (DA) network.

The DA network wraps the celestia-core consensus network by listening for blocks from the consensus network and making them digestible for data availability sampling (DAS).

Continue reading [here](https://blog.celestia.org/celestia-mvp-release-data-availability-sampling-light-clients) if you want to learn more about DAS and how it enables secure and scalable access to Celestia chain data.

## Minimum requirements

| Requirement | Notes          |
|-------------|----------------|
| Go version  | 1.17 or higher |

## System Requirements 

See the official docs page for system requirements per node type: 
* [Bridge](https://docs.celestia.org/nodes/bridge-validator-node#hardware-requirements)
* [Light](https://docs.celestia.org/nodes/light-node#hardware-requirements)
* *Full coming soon*

## Installation

```sh
git clone https://github.com/celestiaorg/celestia-node.git 
cd celestia-node
make install
```

For more information on setting up a node and the hardware requirements needed, go visit our docs at <https://docs.celestia.org>.

## API docs

Celestia-node public API is documented [here](https://docs.celestia.org/developers/node-api/).

## Node types

- **Bridge** nodes - relay blocks from the celestia consensus network to the celestia data availability (DA) network
- **Full** nodes - fully reconstruct and store blocks by sampling the DA network for shares
- **Light** nodes - verify the availability of block data by sampling the DA network for shares

More information can be found [here](https://github.com/celestiaorg/celestia-node/blob/main/docs/adr/adr-003-march2022-testnet.md#legend).

## Run a node

`<node_type>` can be `bridge`, `full` or `light`.

```sh
celestia <node_type> init 
```

```sh
celestia <node_type> start
```

## Package-specific documentation

- [Header](./service/header/doc.go)
- [Share](./service/share/doc.go)
- [DAS](./das/doc.go)

## Code of Conduct

See our Code of Conduct [here](https://docs.celestia.org/community/coc).
