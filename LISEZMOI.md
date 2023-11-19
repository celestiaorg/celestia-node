# Noeud Celestia

[![Référence Go](https://pkg.go.dev/badge/github.com/celestiaorg/celestia-node.svg)](https://pkg.go.dev/github.com/celestiaorg/celestia-node)
[![Version GitHub (dernière version, y compris les préversions)](https://img.shields.io/github/v/release/celestiaorg/celestia-node)](https://github.com/celestiaorg/celestia-node/releases/latest)
[![Go CI](https://github.com/celestiaorg/celestia-node/actions/workflows/go-ci.yml/badge.svg)](https://github.com/celestiaorg/celestia-node/actions/workflows/go-ci.yml)
[![Rapport de Go](https://goreportcard.com/badge/github.com/celestiaorg/celestia-node)]

Implémentation en Golang des types de noeuds de disponibilité des données de Celestia (`léger` | `complet` | `pont`).

Les types de noeuds celestia-node décrits ci-dessus composent le réseau de disponibilité des données (DA) de Celestia.

Le réseau DA enveloppe le réseau de consensus celestia-core en écoutant les blocs du réseau de consensus et en les rendant digestes pour l'échantillonnage de la disponibilité des données (DAS).

Continuez à lire [ici](https://blog.celestia.org/celestia-mvp-release-data-availability-sampling-light-clients) si vous souhaitez en savoir plus sur le DAS et comment il permet un accès sécurisé et évolutif aux données de la chaîne Celestia.

## Table des matières

- [Noeud Celestia](#noeud-celestia)
  - [Table des matières](#table-des-matières)
  - [Exigences minimales](#exigences-minimales)
  - [Configuration requise du système](#configuration-requise-du-système)
  - [Installation](#installation)
  - [Documentation API](#documentation-api)
  - [Types de noeuds](#types-de-noeuds)
  - [Exécuter un noeud](#exécuter-un-noeud)
  - [Variables d'environnement](#variables-denvironnement)
  - [Documentation spécifique au package](#documentation-spécifique-au-package)
  - [Code de conduite](#code-de-conduite)

## Exigences minimales

| Exigence | Remarques        |
| ---------|------------------|
| Version Go | 1.21 ou supérieure |

## Configuration requise du système

Consultez la page officielle de la documentation pour les exigences système par type de noeud :

- [Pont](https://docs.celestia.org/nodes/bridge-node#hardware-requirements)
- [Léger](https://docs.celestia.org/nodes/light-node#hardware-requirements)
- [Complet](https://docs.celestia.org/nodes/full-storage-node#hardware-requirements)

## Installation

```sh
git clone https://github.com/celestiaorg/celestia-node.git
cd celestia-node
make build
sudo make install
```

Pour plus d'informations sur la configuration d'un noeud et les exigences matérielles nécessaires, visitez nos docs sur <https://docs.celestia.org>.

## Documentation API

L'API publique de celestia-node est documentée [ici](https://node-rpc-docs.celestia.org/).

## Types de noeuds

- **Bridge** Noeuds - relaient les blocs du réseau de consensus celestia vers le réseau de disponibilité des données (DA) de celestia
- **Full** Noeuds - reconstruisent entièrement et stockent les blocs en échantillonnant le réseau DA pour des parts
- **Light** Noeuds - vérifient la disponibilité des données de bloc en échantillonnant le réseau DA pour des parts

Vous trouverez plus d'informations [ici](https://github.com/celestiaorg/celestia-node/blob/main/docs/adr/adr-003-march2022-testnet.md#legend).

## Exécuter un noeud

`<node_type>` peut être : `bridge`, `full` or `light`.

```sh
celestia <node_type> init
```

```sh
celestia <node_type> start
```

Veuillez vous référer à [ce guide](https://docs.celestia.org/nodes/celestia-node/) pour plus d'informations sur l'exécution d'un noeud.
## Environment variables

| Variable                | Explication                         | Valeur par défaut	 | Obligatoire |
| ----------------------- | ----------------------------------- | ------------------ | ----------- |
| `CELESTIA_BOOTSTRAPPER` | Démarrer le noeud en mode bootstrap |      `false`       |  Optionnel  |

## Documentation spécifique au package

- [Header](./header/doc.go)
- [Share](./share/doc.go)
- [DAS](./das/doc.go)

## Code de conduite

Consultez notre Code de Conduite [ici](https://docs.celestia.org/community/coc).
