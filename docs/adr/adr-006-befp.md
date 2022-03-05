# ADR #006: Celestia-Node Bad encoding fraud proof(BEFP)

## Changelog

- 2021.03.03 - init commit

## Authors

@vgonkivs @Bidon15 @adlerjohn

## Context

In case when node receive `ErrByzantineRow`/`ErrByzantineCol` from the [rsmt2d](https://github.com/celestiaorg/rsmt2d) library it should generate a fraud proof that gets broadcasted to light clients to inform them that block could be malicious.

## Decision

Started disscussion within:

https://github.com/celestiaorg/celestia-node/issues/4

https://github.com/celestiaorg/celestia-node/issues/263

## Detailed Design
It should be generated when after repairing the entire block, we detect recovered data does not match with its respective row/column roots. The result of RepairExtendedDataSquare will be an error [ErrByzantineRow](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L18-L22)/[ErrByzantineCol](https://github.com/celestiaorg/rsmt2d/blob/f34ec414859fc834835ea97ed54300404eec1ac5/extendeddatacrossword.go#L28-L32):

Both of these errors consist of column/row numbers that do not match with the Merkle root and shares that were successfully repaired and verified(all correct shares).

Using this info we need to prepare a fraud proof that will consist of:
 1. Block height;
 2. Non-nil shares returned with error;
 3. Merkle Proofs of non-nil shares from 2;

```go
type BadEncondingFraudProof struct {
    Height uint64
    Shares [][]byte
    MerkleProofs [][]byte
}
```

After generating a BEFP struct we have to broadcast it to light clients within pub-sub. A BEFP will be generated in ipld/read.go and returned from RetrevieData as an error. In this case `share.Service` should broadcast this error to light clients via separate sub-service `FraudProofService`.

## Status
Proposed

## Consequences

### Positive

Detect incorrectly encoded block

## References

Data Availability(Bad Encoding) Fraud Proofs: [#4](https://github.com/celestiaorg/celestia-node/issues/4)
   
Implement stubs for BadEncodingFraudProofs: [#263](https://github.com/celestiaorg/celestia-node/issues/263) 
