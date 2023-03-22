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
  - [Run using Docker](#run-using-docker)
  - [Run a monitored node](#run-a-monitored-node)
  - [Environment variables](#environment-variables)
  - [Package-specific documentation](#package-specific-documentation)
  - [Code of Conduct](#code-of-conduct)

## Minimum requirements

| Requirement | Notes          |
| ----------- | -------------- |
| Go version  | 1.19 or higher |

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

Celestia-node public API is documented [here](https://docs.celestia.org/developers/node-api/).

## Node types

- **Bridge** nodes - relay blocks from the celestia consensus network to the celestia data availability (DA) network
- **Full** nodes - fully reconstruct and store blocks by sampling the DA network for shares
- **Light** nodes - verify the availability of block data by sampling the DA network for shares

More information can be found [here](https://github.com/celestiaorg/celestia-node/blob/main/docs/adr/adr-003-march2022-testnet.md#legend).

## Run a node

To run a celestia node, you have to first initialize it:
```sh
celestia <node_type> init
```

second, start it:
```sh
celestia <node_type> start
```
`<node_type>` can be: `bridge`, `full` or `light`.

## Run using Docker

Build:
```
make docker-build
```

Run:
```
make docker-run \
    NODE={NODE_TYPE} \
    NETWORK={NETWORK}
```

If you are running multiple nodes on the same machine, you can pass in custom `grpc_port` and `gateway_port` to the docker port mapping to avoid conflicts:

The first node will be on the following ports by default:

- `GRPC_PORT = 9090`
- `GATEWAY_PORT = 26659`

Thus you can port-forward with these ports as starting points, example:

Node 2:

- `GRPC_PORT = 9091`
- `GATEWAY_PORT = 26660`

And so on. And you can pass them to your node as follows:

```
make docker-run \
    NODE={NODE_TYPE} \
    NETWORK={NETWORK} \
    GRPC_PORT=9092 \
    GATEWAY_PORT=26661
```

## Run a monitored node
If you want to run a node with monitoring enabled, you will have to specify so in the `docker-run` make target as follows:
```
$ make docker-run \
    NODE={NODE_TYPE} \
    NETWORK={NETWORK} \
    ENABLE_METRICS=true \
    METRICS_ENDPOINT={METRICS_ENDPOINT} \
    ENABLE_TRACING=true \
    TRACING_ENDPOINT={TRACING_ENDPOINT}
```
Note that you can enable `metrics` and `tracing` separately. For each one of them, you have to specify their corresponding endpoint.
Both the `METRICS_ENDPOINT` and `TRACING_ENDPOINT` should be an open telemetry otel-collector's endpoint.

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
