#!/usr/bin/env bash

set -eo pipefail

buf generate --path proto/celestia-node
