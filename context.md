## Role

You are a **Principal Software Engineer** at Celestia.  
Your objective is to evolve the current **dal-node-rda** (RDA-based fork) into a **Coded Distributed Array (CDA)** system.

## High-Level Context

- The system uses a \(k_1 \times k_2\) **Grid P2P topology**.
- We are moving from **Full Share Replication (RDA)** to **Coded Fragment Storage (CDA)**.
- Goal: achieve **\(1/k\)** storage efficiency using:
  - **Random Linear Network Coding (RLNC)**, and
  - **KZG Validity Proofs**.

## [CRITICAL] Data Flow Architecture

You must implement the following **four-stage data pipeline**.

### Stage 1: Publication (Proposer)

- Generate an \(N \times N\) Extended Data Square (EDS) using the existing `share/eds/rsmt2d.go`.
- Compute **\(N\) KZG Commitments** \((com_1, \dots, com_N)\), one for each column of the piece matrix (where each symbol is treated as \(k\) pieces).
- **Disable Fraud Proofs**:
  - Stop generating *Bad Encoding Fraud Proofs*.
  - KZG commitments now serve as **absolute validity proofs**.

### Stage 2: Initial Distribution (STORE — Cell Level)

- The Publisher identifies the **Custody Cell** \([r, c]\) for each share.
- The Publisher sends the **raw shares** to all nodes currently assigned to that specific cell \([r, c]\).

### Stage 3: Transformation & Intra-Column Gossip (Cell → Column)

- Nodes in cell \([r, c]\), upon receiving raw shares, must:
  - Fragment each share into \(k\) pieces.
  - Independently generate a random coding vector \(g\).
  - Compute a coded fragment \(y = \sum g_i x_i\) and its corresponding **homomorphic KZG proof**.
- Nodes then forward these **coded fragments** to their peers in the same column \(c\).
- Final destination: each peer in column \(c\) verifies and stores **exactly one coded fragment** (achieving \(1/k\) storage).

### Stage 4: Sampling & Reconstruction (Light Client)

- A Light Client (sampler) requests **fragments from random nodes** in column \(c\).
- It verifies each fragment \(y\) using the homomorphic property:
  - \(Com(y) = \sum g_i \, Com(x_i)\).
- Once **\(k\) valid fragments** are collected, it solves the linear system to reconstruct the original share.

## Task Requirements

### 1. Core Logic (Package `share`)

- Refactor **`RDAService`** into **`CDAService`**.
- Implement the logic for **fragmentation** and **RLNC** (using `gnark-crypto`).
- Modify the **STORE protocol** to reflect the **Cell-to-Column** gossip flow.

### 2. Cryptographic Layer (Package `share/crypto`)

- Implement **additively homomorphic KZG**.
- The `ExtendedHeader` must now carry the **\(N\) column commitments** instead of the Merkle root for data availability verification.

### 3. Sampling & Extraction (Packages `das` & `share`)

- Update `das/daser.go` to handle **fragment-based sampling**.
- Implement `share/cda_extractor.go` to perform the **matrix inversion (decoding)** required to recover shares from fragments.

### 4. Integration & Testing

- `nodebuilder`:
  - Register `CDAService` via `fx`.
  - Explicitly **bypass `nodebuilder/fraud` listeners**.
- Testing:
  - Create `share/cda_integration_test.go`.
  - Simulate a Byzantine attack where **20%** of nodes provide invalid fragments.
  - Verify that **KZG filtering** prevents invalid fragments from being stored or used in reconstruction.

## Technical Constraints

- Use **uber-go/fx** for dependency injection.
- Maintain the **single-hop retrieval property** of the Grid topology.
- Ensure the **linear solver** (for reconstruction) is optimized for small \(k\) (e.g., \(k = 16\) or \(k = 32\)).