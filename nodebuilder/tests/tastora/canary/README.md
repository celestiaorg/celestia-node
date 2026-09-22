# Fresh node canary

Can a brand-new light node join the network right now? The canary starts a
light node in Docker with an empty store against a public network, watches what
the node itself reports, restarts it on the same store and fails if any step
does not happen in time. It was written for
[#4842](https://github.com/celestiaorg/celestia-node/issues/4842), after a tail
estimation bug kept fresh Mocha light nodes from starting and nobody noticed
until a user did.

It never submits transactions, never seeds the store and never fetches shares
or blobs itself.

## Checks

| Check | Passes when |
| --- | --- |
| `fresh_store` | The Docker volume is new and owned by this run. |
| `bootstrap` | RPC is up, at least one bootstrapper is connected, head and tail are readable. |
| `head_tail` | The network head is recent and the tail lies inside the storage window. |
| `header_sync` | The tail and its first 100 successors are stored locally and verified as adjacent. |
| `das` | The node's own log shows the DAS worker's first completion for a validated header. |
| `das_recent` (informational) | A block produced after startup arrived by header gossip and was sampled. Never part of the verdict. |
| `restart` | Graceful stop, same container, volume, image and peer ID, retained header, and a new sampling completion after the restart. |
| `cleanup` | Every Docker resource the run created was removed and read back absent. |

A result is `pass`, `fail`, `inconclusive` or `unsupported`. `fail` means the
node missed a milestone within its budget; a node process that exits while the
canary waits for it fails at once with `process_exited` and its exit code in the
evidence. `inconclusive` means the run could not establish evidence (a lost log
stream, a Docker or host problem, a canceled run); it says nothing about the
node, so the test repeats such a run once before it fails. The result also
records timings from process start (startup, header sync, first sample, restart
resume), how many bootstrappers were connected and the header sync rate.

## Evidence

Sampling is judged from the node's own JSON stderr, not from RPC answers: the
`share/light` record `starting sampling session` is joined per data root with
the `das` record `sampled header`, and the header named by that record is
resolved by hash and validated field by field. A `failed to sample header` for
the same height disqualifies the completion, and every witness is derived again
from the sealed log after the node stopped. A log stream that ends while the
node still runs is lost evidence and makes the run `inconclusive`; one that ends
because the node exited delivered everything the node wrote, so the exit is
reported as a node failure and earlier evidence stands.

Two properties of a fresh light node shape the checks:

- The DASer's first job is the tail only, because its checkpoint starts with
  head equal to tail. When that block is empty the availability check returns
  before opening a session, so the node logs one completion without a session
  and has nothing else to sample until a newer head arrives. That completion
  counts for `das` (flagged `empty_block`); `restart` still requires a real
  sampling session, so every passing run contains one.
- Newer heads reach the DASer only through header gossip, and a new peer
  identity can wait minutes for its first gossiped header. `das_recent`
  measures that delay and never decides the outcome.

## Running it

Docker is required. From the repository root:

```sh
make test-fresh-node-canary NETWORK=mocha IMAGE=ghcr.io/celestiaorg/celestia-node:v0.34.2-mocha
```

or directly:

```sh
cd nodebuilder/tests/tastora
CANARY_NETWORK=mainnet CANARY_IMAGE=ghcr.io/celestiaorg/celestia-node:v0.33.0 \
  go test -tags fresh_node_canary -run '^TestFreshNodeCanary$' -count=1 -v -timeout 60m ./canary/
```

`CANARY_IMAGE` may be a tag, a digest or a locally built image. It is pinned
before the run: an image from `ghcr.io/celestiaorg/celestia-node` resolves to
its registry digest, anything else (a local build, an image from a fork) to its
image ID, and the image's `org.opencontainers.image.revision` label names the
commit under test (set it with
`docker build --label org.opencontainers.image.revision=$(git rev-parse HEAD)`
for local builds). `CANARY_REPORT_DIR` writes the full JSON result,
`CANARY_RELEASE` records a release tag in it. A typical passing run takes two
to five minutes.

The [Fresh Node Canary workflow](../../../../.github/workflows/fresh-node-canary.yml)
runs the test:

- nightly, against the newest release of each network (`vX.Y.Z` on Mainnet,
  `vX.Y.Z-mocha` on Mocha);
- when a release is published, against that release's image on the network its
  tag belongs to;
- on pull requests that change the canary;
- on demand, against any published image, for example the build of a `main`
  commit (`ghcr.io/celestiaorg/celestia-node:<7-character sha>`).

A failure posts to Slack like the bootstrapper health check does.

## Layout

| Package | Role |
| --- | --- |
| `canary` | The run: phases, budgets, header checks, witness binding, restart, result assembly. |
| `canary/docker` | One owned container, volume and network per run; image pinning; RPC and log access; cleanup with readback. |
| `canary/telemetry` | Collector for the node's JSON log records and the witness rules. |
| `canary/model` | Result, checks, witnesses, metrics and their validation. |

Unit tests use synthetic sessions and headers and need neither Docker nor a
network: `go test ./canary/...`.
