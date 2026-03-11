# ADR 013: Fibre API

## Authors

@cmwaters

## Changelog

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

This design should preserve that API wherever Fibre semantics match it, while
also exposing Fibre-native functionality that does not fit cleanly into the
blob module.

The design goals are:

- Integrate Fibre blobs into the blob module as much as possible
- Avoid breaking existing blob APIs
- Avoid multiple competing ways to perform the same common task
- Support mixed share versions within the same namespace
- Expose Fibre-specific account and data-plane functionality explicitly

## Decision

Celestia Node will use a hybrid API design:

- The existing blob module will support Fibre share version `2` wherever the
  semantics are compatible with the existing blob API.
- A separate Fibre module will expose Fibre-native operations that are not a
  natural fit for the blob module, including escrow account management and
  Fibre-specific data-plane retrieval.

This means the blob module remains the primary API for chain-visible blob
operations, while the Fibre module provides direct access to Fibre-specific
workflows.

## Detailed Design

### Blob module

The blob module should absorb Fibre support where callers are still working with
chain-visible blobs.

#### Submit

`blob.Submit` continues to accept a list of blobs and routes behavior by share
version:

```go
Submit(ctx, blobs, opts)
```

- Share version `0` or `1`
  - Use the existing `PayForBlob` flow
- Share version `2`
  - Use the Fibre submission flow underneath
  - Encode Fibre rows
  - Upload rows to FSPs
  - Collect validator signatures
  - Submit `MsgPayForFibre`

This keeps Fibre submission aligned with existing blob submission for callers
who are already constructing blobs and choosing a share version.

Note: this will only return height and not the commitment. User will need to
derive the commitment themself.

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

`blob.Get(ctx, height, namespace, commitment)` should not be extended to Fibre
by assumption in this ADR.

The existing method is defined in terms of retrieving a specific on-chain blob
by namespace and commitment from the data square. Fibre's native retrieval model
is different: the client retrieves and reconstructs blob data from FSPs using
the Fibre commitment. It is expected that a user should be ale to retreive the
fibre system level blobs in the namespace.

This ADR therefore leaves `blob.Get` unchanged and treats Fibre support there as
follow-up work only if the method signature and semantics are confirmed to line
up.

### Fibre module

The Fibre module should expose the Fibre-native operations that do not belong in
the blob module.

#### Data-plane operations

The Fibre module should define its public types alongside its methods:

```go
type Namespace = libshare.Namespace

type Commitment [32]byte

type ValidatorSignature []byte

type UploadResult struct {
    Commitment          Commitment
    ValidatorSignatures []ValidatorSignature
    PaymentPromise      *PaymentPromise
    RetentionUntil      *time.Time
}

type SubmitResult struct {
    Commitment          Commitment
    ValidatorSignatures []ValidatorSignature
    Height              uint64
    TxHash              string
    PaymentPromise      *PaymentPromise
    RetentionUntil      *time.Time
}

type PaymentPromise struct {
    ChainID           string
    Namespace         Namespace
    BlobSize          uint32
    Commitment        Commitment
    RowVersion        uint32
    ValsetHeight      uint64
    CreationTimestamp time.Time
    Signature         []byte
}

type Module interface {
    Upload(ctx context.Context, ns Namespace, data []byte) (UploadResult, error)
    Submit(ctx context.Context, ns Namespace, data []byte) (SubmitResult, error)
    Get(ctx context.Context, ns Namespace, commitment Commitment) ([]byte, error)
    Account() AccountModule
}
```

- `Commitment` is the Fibre commitment: `SHA256(rowRoot || rlcOrigRoot)`.
- `UploadResult` captures the artifacts produced by off-chain Fibre upload.
- `SubmitResult` extends `UploadResult` with the on-chain confirmation returned
  after `MsgPayForFibre` submission.
- `PaymentPromise` is surfaced as a first-class type because it is central to
  both staged and full-service Fibre flows.
- `Upload` performs the off-chain Fibre upload flow only. It encodes the blob,
  constructs the payment promise, uploads rows to FSPs, and aggregates
  validator signatures, but does not submit `MsgPayForFibre`.
- `Submit` performs the full Fibre upload flow, including payment promise
  construction, row upload, validator signature aggregation, and
  `MsgPayForFibre` submission.
- `Get` retrieves and reconstructs the original Fibre blob from FSPs by Fibre
  commitment.

These are Fibre-native operations and should not compete with the blob module's
chain-oriented APIs.

`Upload` exists for callers that want direct control over the final on-chain
submission or ordering step, for example to inspect signatures, batch follow-up
work, submit `MsgPayForFibre` through a separate path, or use an ordering
mechanism outside Celestia consensus.

#### Escrow account management

The Fibre module should also expose escrow management methods derived from the
`x/fibre` account interface:

```go
type PendingWithdrawal struct {
    Amount         sdk.Coin
    AvailableAt    time.Time
    CreationHeight uint64
}

type EscrowAccount struct {
    Signer           string
    CurrentBalance   uint64
    AvailableBalance uint64
}

type AccountModule interface {
    QueryEscrowAccount(ctx context.Context, signer string) (*EscrowAccount, error)
    Deposit(ctx context.Context, signer string, amount sdk.Coin) (*EscrowAccount, error)
    Withdraw(ctx context.Context, signer string, amount sdk.Coin) (*PendingWithdrawal, error)
    PendingWithdrawals(ctx context.Context, signer string) ([]PendingWithdrawal, error)
}
```

These functions are Fibre-specific operations and do not belong in
the blob module.

## Rationale

This design balances integration with clarity.

### Preserve one API for overlapping blob workflows

`Submit`, `GetAll`, and `Subscribe` are still fundamentally blob-module
operations, even when share version `2` is involved. Keeping them in `blob`
avoids fragmenting the common application path for submitting blobs and
enumerating namespace contents.

### Avoid forcing Fibre-native behavior into blob APIs

Escrow management and Fibre data-plane retrieval are not generic blob concerns.
Placing them in a dedicated Fibre module keeps the blob API simpler and makes
the Fibre surface explicit.

### Support both full-service and staged Fibre flows

Some callers will want a single high-level `Submit` operation. Others will want to
upload rows, collect validator signatures, and decide themselves whether and how
to order or settle the blob afterwards, including through mechanisms outside
Celestia consensus. Exposing both `Upload` and `Submit` supports both usage
patterns without pushing staged Fibre control flow into the blob module.

### Avoid premature coupling on `blob.Get`

The `blob.Get` signature assumes a specific lookup model based on an on-chain
blob commitment at a given height. Fibre's native `Get` is defined by Fibre
commitment and FSP retrieval semantics. Treating them as the same without
verification would overload the API with ambiguous semantics.

## Consequences

### Positive

- Existing blob users can adopt Fibre incrementally through share version `2`.
- Namespace enumeration and subscription continue to work through the blob
  module across mixed share versions.
- Fibre-specific capabilities are exposed explicitly instead of being hidden in
  blob-module options.
- Callers can choose between a staged `Upload` flow and a full `Submit` flow.
- The API separates chain-visible blob workflows from Fibre-native escrow and
  retrieval workflows.

### Negative

- Fibre functionality is split across two modules.
- `blob.Get` remains asymmetric with `Submit`, `GetAll`, and `Subscribe` until a
  follow-up decision confirms whether Fibre belongs there.
- Implementations must clearly document the difference between on-chain Fibre
  system blobs and Fibre-native reconstructed blob data.

## Alternatives Considered

### Blob-only design

One alternative is to force all Fibre functionality into the blob module.

Rejected because escrow account management and Fibre-native `Upload`, `Submit`,
and `Get` are
not natural blob-module operations and would make the blob API carry
Fibre-specific concepts.

### Fully separate Fibre API

Another alternative is to introduce a completely separate Fibre API for submit,
enumeration, and subscription.

Rejected because it would duplicate the blob module's role for common
operations, fragment the user experience, and make mixed-version namespaces more
awkward to work with.

### Extend `blob.Get` immediately

Another alternative is to define Fibre support for `blob.Get` now.

Rejected for now because the lookup key and retrieval semantics have not yet
been shown to match the current blob API cleanly.

## References

- [ADR 009: Public API](./adr-009-public-api.md)
- [Blob service implementation](../../blob/service.go)
- [Blob type and share version handling](../../blob/blob.go)
- [Client Blob API usage examples](../../api/client/readme.md)
- [go-square Fibre share version 2 layout](../../../go-square/README.md)
- [Fibre client API](../../../fibre-da-spec/client.md)
- [Fibre encoding specification](../../../fibre-da-spec/encoding.md)
- [Fibre SDK escrow module](../../../fibre-da-spec/sdk_fibre_module.md)
