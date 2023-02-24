# ADR 13: Tail Padding Optimizations

## Changelog

- 24.02.2023: First Draft

## Author

@nashqueue

## Context

If the Celestia block is not full it will create tail padding. Tail padded shares look always the same. Tail padded shares create tail padded rows. tail padded rows erasure to erasure-coded tail padded rows. Tail padded rows combined with erasure-coded tail padded rows result in a full tail padded row. A full tail padded row of the same length will always have the same Namespace Merkle Trie on top of it. This will result in a tail padded row root.
All of those things are deterministic and can be computed beforehand.

Given a tail padded row root inside the DA-Header you can deduce that the underlying row is a full tail padded row. This information can be beneficial in many ways.

### DAS

If a Light Node receives a tail padded row root in the DAH it does not have to DAS this root. It knows the underlying shares and could generate the shares and path itself.

Hypothesis: The more tail padded rows exist the fewer samples a Light Node has to do for the same DA guarantee.

### Block reconstruction

Given a tail padded rowroot you can use the predetermined shares to fill up the store. You will not need to request those shares from light nodes. This will save bandwidth and reconstruction time.

### Storage

You don`t need to save copies of the same shares multiple times. Given that 3/4 of the ODS can be tail padding it can save up to 3/16 + 3/16 = 6/16 = 3/8 of storage of the EDS.

### Bandwidth

You don`t need to share tail-padded rows over the wire. Given the root, the recipient can deduce the whole row.

## Decision for each Optimization

- DAS:
- Block reconstruction:
- Storage:
- Bandwidth:

## Status

Proposed

## Consequences

### Positive

Less Bandwith, Storage, and  Computation required.

### Negative

### Neutral
