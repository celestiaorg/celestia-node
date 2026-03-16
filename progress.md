# Progress Log

This file tracks implementation progress for the CDA refactor described in `context.md`.

## 2026-03-16

- Added CDA config flags in `nodebuilder/share/config.go`: `CDAEnabled`, `CDAK`, `CDAUseRDAFallback`.
- Added homomorphic KZG interface scaffold in `share/crypto/kzg_homomorphic.go`.
- Added CDA fragment types in `share/cda_fragment.go`:
  - `FragmentID` now uses `uint16` for `Row`/`Col`.
  - Added `CanonicalKey()` + `ParseFragmentKey()` for KV storage layout.
- Added CDA key parsing error in `share/cda_errors.go`.
- Added initial in-memory `CDAService` storage-policy core in `share/cda_service.go`:
  - Peer-ID binding (1 fragment per peer per share coordinate).
  - Hard cap (`K + Buffer`) per share coordinate.
- Added CDA pubsub scaffolding:
  - `share/cda_subnet.go`: `CDASubnetManager` joins/subscribes to `cda/cell/<row>/<col>` and `cda/col/<col>`.
  - `share/cda_messages.go`: JSON-encoded message envelope for raw-share STORE and fragment gossip (placeholder until Protobuf messages are introduced).
- Added `CDAService.ValidateAndStoreFragment(...)` to enforce **KZG validation gate** before anti-spam storage policy.
- Added CDA node wiring scaffold:
  - `share/cda_node.go`: `CDANode` binds pubsub receivers to KZG-verify + storage policy using a `ColumnCommitmentProvider`.
- Added seeded RLNC coding vector helpers:
  - `share/cda_rlnc.go`: `NewSeededCodingVector(k)` and `MaterializeCodingVectorCoeffs(...)`.
- Implemented minimal Stage 2→3 scaffold:
  - `CDANode` now converts `CDAStoreMessage` into a `CDAFragmentMessage` and publishes to `cda/col/<col>` (proof generation pending).
- Improved JSON encoding footprint for seeded coding vectors:
  - `share/crypto/coding_vector_json.go`: custom JSON marshal/unmarshal encodes `Seed` and `Coeff` as base64 strings instead of large integer arrays.
- Added development stubs for early CDA integration:
  - `share/crypto/noop_kzg.go`: `NoopHomomorphicKZG` (shape checks only; not secure).
  - `share/cda_commitments.go`: `StaticColumnCommitments` as a temporary `ColumnCommitmentProvider` until header commitments are wired.
- Wired CDA into nodebuilder (behind `CDAEnabled`):
  - `nodebuilder/share/cda_module.go`: fx wiring for `CDANode` using noop KZG + static commitments.
  - `nodebuilder/share/module.go`: includes `cdaComponents(tp, cfg)` in share module construction.
- Added CDA buffer configuration:
  - `nodebuilder/share/config.go`: `CDABuffer` with default `4` (hard cap = `k + buffer`).
  - `nodebuilder/share/cda_module.go`: uses `CDABuffer` when provided; otherwise defaults to `4`.
- Implemented Protobuf wire protocol for CDA pubsub payloads:
  - `share/cda/pb/cda.proto` + generated `share/cda/pb/cda.pb.go`.
  - Installed `protoc-gen-gogofaster` to generate CDA pb Go code.
  - `share/cda_messages.go` switched from JSON envelope to protobuf-framed messages (1-byte tag + protobuf payload) for P2P.
  - Note: project-wide `make pb-gen` currently regenerates other protobufs with upstream `go_package` paths that may not match module versions (e.g. celestia-app `/v7`). We reverted unrelated regenerated pb.go files to keep the repo building; CDA pb generation can be run in isolation for now.
- Implemented the CDA decoding “engine” (linear solver):
  - `share/cda_extractor.go`: Gaussian elimination solver for `G * x = y` over BLS12-381 field.
  - `share/cda_extractor_test.go`: identity/non-singular/singular matrix tests.
- Implemented the CDA validation “shield” (real group math via gnark-crypto):
  - `share/crypto/homomorphic_kzg_bls12381.go`: `BLS12381HomomorphicKZG` (homomorphic G1 commitments; transitional proof semantics).
  - `share/crypto/homomorphic_kzg_bls12381_test.go`: combine/verify tests.
  - `nodebuilder/share/cda_module.go`: switched verifier from noop to `NewBLS12381HomomorphicKZG()` (commitments provider still stubbed).
- Added an end-to-end pubsub test that exercises the real shield:
  - `share/cda_shield_integration_test.go`: publishes a fragment with correct combined-commitment proof and verifies the node stores it; then publishes a fragment with wrong proof and verifies it is dropped at the validation gate.
- Bridged CDA with actual blockchain headers (commitments source):
  - `header/header.go`: added `CDAH *CDAHeader` to `ExtendedHeader` and `CDAHeader` struct containing `ColumnCommitments`.
  - `header/pb/extended_header.proto` + regenerated `header/pb/extended_header.pb.go`: added `CDAHeader` message and `cdah` field to protobuf encoding.
  - `header/serde.go`: marshal/unmarshal CDA commitments through protobuf.
  - `share/cda_header_provider.go`: `HeaderColumnCommitmentProvider` fetches commitments by block height from the header service/store.
  - `nodebuilder/share/cda_module.go`: wiring now uses `HeaderColumnCommitmentProvider` (no more static commitments stub).
- Bypassed bad-encoding fraud proof broadcasting when CDA commitments are present:
  - `das/daser.go`: if `h.CDAH` exists, do not broadcast BEFP on byzantine sampling errors.
- DAS integration (CDA path via Availability wiring):
  - `share/availability/cda/availability.go`: new CDA-based `share.Availability` that samples coordinates and reconstructs from `k` stored fragments using `share.SolveLinearSystem` (bypasses Merkle/fraud).
  - `nodebuilder/share/module.go`: when `CDAEnabled` on Light nodes, provide CDA availability instead of light sampling availability.
- E2E reconstruction test (fragments + header commitments only):
  - `nodebuilder/tests/cda_reconstruct_test.go`: stores `k` valid fragments from distinct peers and asserts `cda` availability succeeds (no merkle proofs, no fraud proofs).
- Added robustness tests for CDA pubsub + validation gate:
  - `share/cda_pubsub_test.go`: simulates out-of-order delivery with concurrent publishes, duplicate-per-peer spam, and byzantine fragments (wrong data or wrong proof); asserts they are dropped at the KZG validation gate and checks hard cap `K+Buffer` is enforced.

