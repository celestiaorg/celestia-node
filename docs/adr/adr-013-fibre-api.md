# ADR 013: Fibre API

## Authors

@cmwaters @vgonkivs

## Changelog

- 2026-04-10: Rework input param from commitment to blob ID; apply review feedback
- 2026-03-10: Initial version

## Status

Proposed

## Context

Celestia is introducing Fibre, a new data availability mechanism that disperses
blob data across Fibre Service Providers (FSPs) using erasure coding,
validator attestations, and a pre-funded escrow model.

Fibre differs from the existing blob flow in several important ways:

- Blob data is uploaded to FSPs instead of being stored directly in the data
  square.
- Retrieval reconstructs the original blob from rows fetched from FSPs.
- A Fibre blob is represented on-chain by a system-level share version `2`
  blob.
- Payment is based on escrow accounts and `MsgPayForFibre`, rather than only on
  the existing `PayForBlob` flow.

At the same time, Fibre overlaps substantially with the existing blob UX:

- Users still submit data under namespaces.
- Included Fibre blobs still appear on-chain as namespaced blob data.
- Callers still need to enumerate blobs in namespaces and watch new blobs as
  they arrive.

The existing `celestia-node` blob module already exposes the main operations
applications use today:

- `Submit`
- `Get`
- `GetAll`
- `Subscribe`

This design should preserve that API for read-path operations (`GetAll`,
`Subscribe`) wherever Fibre semantics match, while exposing Fibre-native
functionality — including submission — through a dedicated Fibre module.

The design goals are:

- Integrate Fibre blobs into the blob module as much as possible
- Avoid breaking existing blob APIs
- Avoid multiple competing ways to perform the same common task
- Support mixed share versions within the same namespace
- Expose Fibre-specific account and data-plane functionality explicitly

## Decision

Celestia Node will use a hybrid API design:

- The existing blob module will support Fibre share version `2` on the read
  path (`GetAll`, `Subscribe`) wherever the semantics are compatible with the
  existing blob API.
- A separate Fibre module will expose all Fibre-native operations, including
  blob submission (`Submit`, `Upload`), data-plane retrieval (`Get`), and
  escrow account management (`Deposit`, `Withdraw`, `QueryEscrowAccount`,
  `PendingWithdrawals`) as a flat interface.

This means the blob module remains the primary API for reading chain-visible
blob data, while the Fibre module owns the full Fibre submission and retrieval
lifecycle.

## Detailed Design

### Blob module

The blob module remains unchanged for Fibre. Fibre submission is handled
entirely through the dedicated Fibre module rather than routing by share version
inside `blob.Submit`.

#### GetAll

`blob.GetAll` should support mixed namespaces that contain share versions `0`,
`1`, and `2`:

```go
GetAll(ctx, height, namespaces)
```

For share version `2`, the blob module returns the on-chain system-level Fibre
blob, because that is what is actually committed into the data square. This
keeps `GetAll` consistent with its current role as a namespaced on-chain blob
enumeration API.

#### Subscribe

`blob.Subscribe` should also support mixed namespaces and emit share version `2`
blobs when a `MsgPayForFibre` results in a Fibre system blob being included:

```go
Subscribe(ctx, namespace)
```

This preserves the existing subscription model for clients that watch namespace
activity without requiring them to learn a second streaming API.

#### Get

`blob.Get(ctx, height, namespace, commitment)` is not extended to Fibre in this
ADR. The existing method retrieves a specific on-chain blob by namespace and
commitment from the data square. Fibre's native retrieval model is different:
the client retrieves and reconstructs blob data from FSPs using a blob ID. It is
expected that a user should be able to retrieve the Fibre system-level blobs in
the namespace via `GetAll`.

This ADR therefore leaves `blob.Get` unchanged and treats Fibre support there as
follow-up work only if the method signature and semantics are confirmed to line
up.

### Fibre module

The Fibre module exposes Fibre-native operations that do not belong in the blob
module, including blob upload and submission, data-plane retrieval, and escrow
account management — all as a flat interface.

#### Data-plane operations

The Fibre module defines its public types alongside its methods:

```go
type ValidatorSignature []byte

type UploadResult struct {
    BlobID              appfibre.BlobID
    ValidatorSignatures []ValidatorSignature
    PaymentPromise      *PaymentPromise
}

type SubmitResult struct {
    BlobID              appfibre.BlobID
    ValidatorSignatures []ValidatorSignature
    Height              uint64
    TxHash              string
    PaymentPromise      *PaymentPromise
}

type GetBlobResult struct {
    Data []byte
}

type PaymentPromise struct {
    ChainID           string
    Namespace         libshare.Namespace
    BlobSize          uint32
    Commitment        appfibre.Commitment
    RowVersion        uint32
    ValsetHeight      uint64
    CreationTimestamp  time.Time
    Signature         []byte
}

type Module interface {
    Submit(ctx context.Context, ns libshare.Namespace, data []byte, cfg *txclient.TxConfig) (*SubmitResult, error)
    Upload(ctx context.Context, ns libshare.Namespace, data []byte, cfg *txclient.TxConfig) (*UploadResult, error)
    Get(ctx context.Context, blobID []byte) (*GetBlobResult, error)
    QueryEscrowAccount(ctx context.Context, signer string) (*EscrowAccount, error)
    Deposit(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error
    Withdraw(ctx context.Context, amount sdktypes.Coin, cfg *txclient.TxConfig) error
    PendingWithdrawals(ctx context.Context, signer string) ([]PendingWithdrawal, error)
}
```

- `BlobID` is the Fibre blob identifier (containing both the commitment and
  namespace information) used to upload, submit, and retrieve blobs.
- `UploadResult` captures the artifacts produced by off-chain Fibre upload.
- `SubmitResult` extends `UploadResult` with the on-chain confirmation returned
  after `MsgPayForFibre` submission.
- `GetBlobResult` wraps the raw blob data reconstructed from FSPs.
- `PaymentPromise` is surfaced as a first-class type because it is central to
  both staged and full-service Fibre flows.
- `Upload` performs the Fibre upload flow optimized for low latency. It encodes
  the blob, constructs the payment promise, uploads rows to FSPs, and aggregates
  validator signatures. It should also submit `MsgPayForFibre` in the background
  to settle the payment, as long as this does not add latency to the Upload call
  itself. Upload returns as soon as the off-chain upload and signature collection
  is complete (~1 network round trip), without waiting for on-chain confirmation.
  This is the preferred method for latency-sensitive callers.
- `Submit` performs the full Fibre upload flow and waits for on-chain
  confirmation. It includes payment promise construction, row upload, validator
  signature aggregation, and `MsgPayForFibre` submission, blocking until the
  transaction is included in a block. This provides the caller with the
  confirmation height, at the cost of higher latency (~2x block time).
- `Get` retrieves and reconstructs the original Fibre blob from FSPs by Fibre
  commitment.

These are Fibre-native operations and should not compete with the blob module's
chain-oriented APIs.

`Upload` exists for callers that prioritize low latency over on-chain
confirmation. Because it returns after off-chain upload without waiting for
block inclusion, a caller gets a response in roughly one network round trip to
FSPs, compared to `Submit` which adds at least one block time for `MsgPayForFibre`
confirmation. This makes `Upload` the right choice for latency-sensitive
applications that do not need to know the confirmation height immediately.

`Upload` should submit `MsgPayForFibre` in the background to ensure that the
Fibre payment is settled without requiring the caller to handle it. This avoids
relying on a separate timeout agent as the primary payment mechanism and keeps
the caller's interaction simple: one call, data is available, payment is handled.
The background submission must not block or delay the Upload response.

#### Escrow account management

Escrow account operations are exposed directly on the Fibre module interface
rather than via a nested `AccountModule`. This keeps the API surface flat and
avoids unnecessary indirection for callers:

```go
type EscrowAccount struct {
    Signer           string
    Balance          sdktypes.Coin
    AvailableBalance sdktypes.Coin
}

type PendingWithdrawal struct {
    Signer             string
    Amount             sdktypes.Coin
    RequestedTimestamp time.Time
    AvailableTimestamp time.Time
}
```

- `QueryEscrowAccount` returns the escrow account details for a given signer.
- `Deposit` and `Withdraw` resolve the signer from `TxConfig` (via
  `SignerAddress`, `KeyName`, or the node's default account) rather than
  accepting an explicit signer string. This is consistent with how other
  transaction-submitting methods work across celestia-node.
- `Deposit` and `Withdraw` return only an error, keeping the interface minimal.
- `PendingWithdrawals` returns all pending (not yet claimable) withdrawals for
  a given signer.

These functions are Fibre-specific operations and do not belong in
the blob module.

## Rationale

This design balances integration with clarity.

### Preserve one API for overlapping blob read workflows

`GetAll` and `Subscribe` are still fundamentally blob-module operations, even
when share version `2` is involved. Keeping them in `blob` avoids fragmenting
the common application path for enumerating namespace contents.

### Keep Fibre submission separate from blob.Submit

Rather than routing `blob.Submit` by share version, Fibre submission lives
entirely in the Fibre module. This avoids overloading the blob submission path
with Fibre-specific concerns (escrow, payment promises, FSP interaction) and
keeps the blob module focused on `PayForBlob` semantics.

### Avoid forcing Fibre-native behavior into blob APIs

Escrow management and Fibre data-plane retrieval are not generic blob concerns.
Placing them in a dedicated Fibre module keeps the blob API simpler and makes
the Fibre surface explicit.

### Support both low-latency and confirmed Fibre flows

Fibre enables two distinct latency profiles:

- **`Upload` (~1 round trip)**: The blob is erasure coded, uploaded to FSPs, and
  validator signatures are collected. The caller gets a response as soon as the
  data is available on the DA layer, without waiting for Celestia consensus.
  `MsgPayForFibre` is submitted in the background to settle the payment. This is
  the lowest latency path and is the main advantage Fibre offers over the
  existing `PayForBlob` flow.

- **`Submit` (~2x block time)**: Same as Upload, but blocks until `MsgPayForFibre`
  is confirmed on-chain. The caller gets the confirmation height, which is
  needed for applications that require Celestia consensus ordering.

Having both methods avoids forcing latency-sensitive callers to wait for chain
confirmation they don't need, while still providing a simple single-call path
for callers that do need ordering or finality.

Note: `Upload` submitting `MsgPayForFibre` in the background is a deliberate
design choice. The payment must be settled to avoid losing escrow funds, and
making this automatic removes the burden from callers. This is not the same as
relying on a timeout agent — the PFF is submitted proactively after every
upload, the agent is only a backup for edge cases where submission fails.

### Flat module interface over nested sub-modules

Escrow account operations are exposed directly on the Fibre module interface
rather than through a nested `AccountModule`. This reduces indirection, keeps
the RPC surface simpler, and matches the pattern used by other celestia-node
modules.

### Avoid premature coupling on `blob.Get`

The `blob.Get` signature assumes a specific lookup model based on an on-chain
blob commitment at a given height. Fibre's native `Get` is defined by blob ID
and FSP retrieval semantics. Treating them as the same without verification
would overload the API with ambiguous semantics.

## Consequences

### Positive

- Namespace enumeration and subscription continue to work through the blob
  module across mixed share versions.
- Fibre-specific capabilities are exposed explicitly instead of being hidden in
  blob-module options.
- Callers can choose between a staged `Upload` flow and a full `Submit` flow.
- The API separates chain-visible blob workflows from Fibre-native escrow and
  retrieval workflows.
- `Fibre.Get` does not require a height, simplifying the retrieval API. The
  validator set is not expected to change enough during a blob's retention period
  to affect retrievability.
- Flat module interface avoids unnecessary nesting for RPC consumers.

### Negative

- Fibre submission and blob submission are separate code paths; callers must
  choose between `blob.Submit` and `fibre.Submit`.
- `blob.Get` remains asymmetric with `GetAll` and `Subscribe` until a follow-up
  decision confirms whether Fibre belongs there.
- Implementations must clearly document the difference between on-chain Fibre
  system blobs and Fibre-native reconstructed blob data.

## Alternatives Considered

### Blob-only design

One alternative is to force all Fibre functionality into the blob module.

Rejected because escrow account management and Fibre-native `Upload`, `Submit`,
and `Get` are not natural blob-module operations and would make the blob API
carry Fibre-specific concepts.

### Route blob.Submit by share version

Another alternative is to have `blob.Submit` detect share version `2` and
automatically route to the Fibre submission flow underneath.

Rejected because Fibre submission has fundamentally different concerns (escrow
funding, payment promises, FSP interaction, validator signatures) that would
complicate the blob submission path. Keeping Fibre submission in its own module
makes these concerns explicit and avoids coupling `blob.Submit` to Fibre
internals.

### Fully separate Fibre API

Another alternative is to introduce a completely separate Fibre API for submit,
enumeration, and subscription.

Rejected because it would duplicate the blob module's role for common
operations, fragment the user experience, and make mixed-version namespaces more
awkward to work with.

### Nested AccountModule sub-interface

Another alternative is to expose escrow operations via a nested
`Account() AccountModule` method on the Fibre module.

Rejected in favor of a flat interface because the additional indirection adds
complexity without meaningful benefit, and the flat pattern is more consistent
with how other celestia-node modules expose their APIs.

### Extend `blob.Get` immediately

Another alternative is to define Fibre support for `blob.Get` now.

Rejected for now because the lookup key and retrieval semantics have not yet
been shown to match the current blob API cleanly.

## References

- [ADR 009: Public API](./adr-009-public-api.md)
- [Blob service implementation](../../blob/service.go)
- [Blob type and share version handling](../../blob/blob.go)
- [Client Blob API usage examples](../../api/client/readme.md)
- [go-square Fibre share version 2 layout](https://github.com/celestiaorg/go-square/blob/main/README.md)
- [celestia-app Fibre core types (`PaymentPromise`, `EscrowAccount`, `Withdrawal`)](https://github.com/celestiaorg/celestia-app/blob/feature/fibre/proto/celestia/fibre/v1/fibre.proto)
- [celestia-app Fibre query API (`EscrowAccount`, `Withdrawals`, `ValidatePaymentPromise`)](https://github.com/celestiaorg/celestia-app/blob/feature/fibre/proto/celestia/fibre/v1/query.proto)
- [celestia-app Fibre tx API (`MsgPayForFibre`, escrow deposit, withdrawal)](https://github.com/celestiaorg/celestia-app/blob/feature/fibre/proto/celestia/fibre/v1/tx.proto)
