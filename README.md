# Celestia Node

[![Go Reference](https://pkg.go.dev/badge/github.com/celestiaorg/celestia-node.svg)](https://pkg.go.dev/github.com/celestiaorg/celestia-node)
[![GitHub release (latest by date including pre-releases)](https://img.shields.io/github/v/release/celestiaorg/celestia-node)](https://github.com/celestiaorg/celestia-node/releases/latest)
[![test](https://github.com/celestiaorg/celestia-node/actions/workflows/test.yml/badge.svg)](https://github.com/celestiaorg/celestia-node/actions/workflows/test.yml)
[![lint](https://github.com/celestiaorg/celestia-node/actions/workflows/lint.yml/badge.svg)](https://github.com/celestiaorg/celestia-node/actions/workflows/lint.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/celestiaorg/celestia-node)](https://goreportcard.com/report/github.com/celestiaorg/celestia-node)
[![codecov](https://codecov.io/gh/celestiaorg/celestia-node/branch/main/graph/badge.svg?token=CWGA4RLDS9)](https://codecov.io/gh/celestiaorg/celestia-node)


Golang implementation of Celestia's data availability node types (`light` | `full` | `bridge`).

The celestia-node types described above comprise the celestia data availability (DA) network. 

The DA network wraps the celestia-core consensus network by listening for blocks from the consensus 
network and making them digestible for data availability sampling (DAS). 

Continue reading [here](https://blog.celestia.org/celestia-mvp-release-data-availability-sampling-light-clients)
if you want to learn more about DAS and how it enables secure and scalable access to celestia chain data.

### Minimum requirements

| Requirement | Notes            |
|-------------|------------------|
| Go version  | 1.17 or higher   |

### Installation

```sh
git clone https://github.com/celestiaorg/celestia-node.git 
```
```
make install
```

### Node types
* **Bridge** nodes - relay blocks from the celestia consensus network to the celestia data availability 
(DA) network
* **Full** nodes - fully reconstruct and store blocks by sampling the DA network for shares
* **Light** nodes - verify the availability of block data by sampling the DA network for shares

More information can be found [here](https://github.com/celestiaorg/celestia-node/blob/main/docs/adr/adr-003-march2022-testnet.md#legend).

### Run a node

`<node_type>` can be `bridge`, `full` or `light`.

```sh
celestia <node_type> init 
```
```sh
celestia <node_type> start
``` 

### Package-specific documentation
* [Header](./service/header/doc.go)
* [Share](./service/share/doc.go)
* [DAS](./das/doc.go)
