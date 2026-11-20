# Celestia devnet bridge image

This image packages the `celestia bridge` service used by the two-container devnet in `celestia-app`. It preserves that bridge image's startup, health, `/credentials`, environment, port, and `/data/bridge` interfaces.

The public image is `ghcr.io/celestiaorg/celestia-devnet-bridge:<celestia-node-tag>`. It is built for `linux/amd64` and `linux/arm64` when a `v*` tag is pushed in this repository. Architecture-specific tags use `<celestia-node-tag>-amd64` and `<celestia-node-tag>-arm64`.

Use the image as the bridge service alongside the compatible validator image from `celestia-app`. The bridge reads `node-0` from `/credentials` and uses `CORE_HOST`, `CORE_RPC_PORT`, and `CORE_GRPC_PORT` to connect to the validator. Mount persistent storage at `/data/bridge`.

`/credentials` contains `node-0` through `node-9` and `validator-0` `.key` (password `password`), `.plaintext-key`, and `.addr` files, plus `validator-0.valaddr`. These are disposable devnet credentials only.

Run `bash docker/devnet/bridge.sh` and `bash docker/devnet/health.sh` for the image entrypoint and health check implementations. Publication can be rerun from **Actions → devnet → Run workflow** for an existing node release tag.
