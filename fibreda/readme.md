# FibreDA — Full Specification v1.0 (client & server)

---

## 0) Glossary

* **FSP**: Fibre Service Provider — a validator-operated server storing rows.
* **Row**: fixed-size 2 KiB chunk used by rsema1d; contains encoded data.
* **Commitment**:  SHA256(rowRoot || rlcRoot)
* **RLC**/**rlc\_orig**: GF(2^128) vector used for rsema1d **Context-Based Verification**.
* **PFB**: PayForBlob (on-chain transaction), used to confirm & promote storage.
* **Promotion**: extending TTL from *unconfirmed* (\~5m) to *confirmed* (\~24h) upon PFB.
* **Attestation**: validator signature binding commitment to a service-period minute.
* **ShardMap**: deterministic mapping from (commitment, row index, validator) → assigned set.

---

## 1) Goals & non-goals

**Goals**

1. Encode a blob into rows (2 KiB), **shard** across FSPs, and collect ≥2/3 **attestations** by count and voting power.
2. **Submit PFB** with the commitment and original length; servers **promote** on PFB.
3. Provide **bulk** upload path for v1 (simple, fits ≤\~300 KiB per validator) and keep **streaming** as a ready fallback for larger payloads.
4. Keep v1 client **library-first** for ease of testing; support a **light-node** mode later.

**Non-goals (v1)**

* Shipping a SQL or custom epoch-bucket store; v1 is **Badger-only**. We will test SQL-light and epoch-bucket variants later and choose by measured performance.

---

## 2) Parameters & reasoning

| Parameter            |           Default | Derived/Notes                                   |
| -------------------- | ----------------: | ----------------------------------------------- |
| `row_size`           |         **2 KiB** | Balance proof overhead vs sharding granularity. |
| `max_rows`           | **65,536 (2^16)** | 16-bit indices; `max_blob_size ≈ 128 MiB`.      |
| `encoding_factor`    |             **1** | Parity = originals (K==N). Tunable later.       |
| `val_set_size`       |           **100** | Planning default; runtime uses actual.          |
| `rows_per_val`       |           **150** | Per-FSP payload ≈ **300 KiB** (good for bulk).  |
| `replication_factor` |           **TBD** | See §11.1; impacts `rows_per_val`.              |
| `max_blob_size`      |      **≈128 MiB** | `row_size * max_rows`.                          |
| `rlc_orig_size`      |        **≈1 MiB** | `16 * max_rows` GF128 coeffs.                   |

**Server TTLs & limits**

* `unconfirmed_ttl = 5m`
* `safety_buffer = 1m`
* `confirmed_ttl = 24h`
* `server_rows_per_message_limit = rows_per_val + 1 (=151)`
* `val_throughput ≈ 10 MiB/s` ⇒ worst-case **\~33 RPS** at **300 KiB** per request.

**Client concurrency**: `send_workers=20`, `read_workers=20`.

---

## 3) Design decisions (summary)

* **Bulk vs Streaming**: **Bulk chosen** in v1 because per-FSP payload ≈ **300 KiB**; simpler state machine. **Streaming kept** for larger payloads or strict proxies. (§6)
* **Constructor modes**: **Library-first** in v1 to avoid sync/storage; **Light-node** mode kept as an option. (§4)
* **ValTracker**: embeds **HeaderService**, fetches endpoints from a **provider records server**, supports **Active** (subscribe) and **Lazy** (on-demand). (§4.2)
* **Concurrency**: run **rsema1d encoding in parallel** with ValTracker refresh (computationally heavy). (§4.3)
* **Server verification**: strict pipeline **dedup → row proof → RLC context check → persist → sign**. RLC check thwarts adversarial encodings. (§8.3)
* **Promotion listener**: chain watcher that promotes TTL and may return a **new attestation**. (§8.5)
* **Backpressure**: conservative **10 MiB/s** cap ⇒ **\~33 RPS**, configurable; explain rationale. (§8.8)
* **Refresh semantics**: `Refresh(commitment)` = **submit PFB again** to extend TTL **without re-uploading rows**. (§5.3)

---

## 4) Client architecture

### 4.1 Construction & config

Two explicit constructors:

```go
// Light-node backed (headers stream available; good for Active ValTracker):
NewClientWithLightNode(cfg LightNodeConfig, vtMode ValTrackerMode, opts ...Option) (*Client, error)

// Library-first (preferred v1; no sync/storage overhead; ValTracker defaults to Lazy):
NewClientWithLibrary(cfg LibraryConfig, vtMode ValTrackerMode, opts ...Option) (*Client, error)
```

**Options**: `WithSendWorkers(int)`, `WithReadWorkers(int)`, `WithShardMap(ShardMap)`, `WithPFBSubmitter(PFBSubmitter)`, `WithDialer(Dialer)`.

**Why library-first in v1?** Faster to test, no local chain sync or storage, fewer moving parts.

### 4.2 ValTracker (Active vs Lazy)

* **Inputs**: `HeaderService.NetworkHead()` for active set; **Provider Records Server** for FSP IP/port.
* **Active Mode**: subscribe to head updates; on changes, refresh endpoints and connection pool (best with light node).
* **Lazy Mode**: resolve only when needed (`Put/Get/Refresh`), minimizing background traffic (best with client library mode).

```go
type ValInfo struct { PubKey []byte; VotingPow uint64; Address string }

type ValTracker interface { ActiveSet(ctx context.Context) ([]ValInfo, error) }
```

### 4.3 Concurrency pipeline

* Start **ValTracker refresh** and **rsema1d encoding** **concurrently**.
* When both ready: compute **ShardMap**, start **bulk uploads** with `send_workers`.

### 4.4 Public API

```go
type Client interface {
  Put(ctx context.Context, data []byte) (commitment [32]byte, pfbHeight uint64, err error)
  Get(ctx context.Context, commitment [32]byte, originalLength uint64) ([]byte, error)
  Refresh(ctx context.Context, commitment [32]byte) (pfbHeight uint64, err error) // PFB resubmission only
}
```

### 4.5 Sharding & replication

* **Default ShardMap**: **SipHash‑2‑4** keyed by `commitment` over `(val_pubkey || row_index)`; per validator pick the lowest `rows_per_val` scores.
* **Replication** (TBD): deterministic top‑`R` scores per row, or probabilistic cutoff. See §11.1.

---
#### ShardMap details
1. SipHash-2-4

* **SipHash** is a simple, fast, keyed hash function.
* “2-4” means the variant with **2 compression rounds** and **4 finalization rounds** (a common secure choice).
* It outputs a 64-bit pseudorandom score.

2. Keyed by `commitment`

* Each blob has a unique **commitment** (32 bytes).
* We use it as the *secret key* to SipHash.
  → This makes the sharding plan unpredictable without the actual blob.

3. Input = `(val_pubkey || row_index)`

* For each **validator** (`val_pubkey`) and each **row** (`row_index`), we concatenate them and feed them into SipHash (with the blob’s commitment as the key).
* Output = 64-bit score that is stable and deterministic for this `(validator, row)` pair.

4. Per validator → lowest `rows_per_val` scores

* Each validator will “own” a fixed number of rows (`rows_per_val`, e.g. 150).
* To pick them:

    1. Compute SipHash score for every row index with that validator’s pubkey.
    2. Sort by score.
    3. Take the **lowest `rows_per_val`** scores.
    4. Those row indices are assigned to that validator.


---

## 5) Client flows

### 5.1 Put()

0. Bounds: `0 < len(data) ≤ 128 MiB`.
1. Valset: `vals := vt.ActiveSet()`.
2. Encode (parallel with 1): rsema1d → `commitment`, `rlc_orig`, `rows[]`, `proofs[]`.
3. Plan: `assign := ShardMap(len(rows), rows_per_val, commitment, vals)`.
4. Upload bulk (parallel per FSP; ≤ `send_workers`): send `UploadRowsRequest{commitment, rlc_orig, rows subset+proofs}`.
5. Collect **attestations**; verify ed25519 over domain‑tagged digest; aggregate by **count** and **voting power**.
6. On ≥2/3 quorum (both): **Submit PFB** with `{commitment, original_length}`.
7. Return `(commitment, height)`.

**Note**: Bulk chosen because each FSP’s share ≈ **300 KiB**. If payloads grow or proxies enforce small limits, switch to streaming (§6.2).

### 5.2 Get()

1. Valset via ValTracker.
2. Plan with ShardMap (who should have what).
3. Fetch in parallel (`read_workers`) by **bitmap** or **indices**; request only missing indices; de‑dup sources.
4. Verify each row’s **Merkle proof** against `commitment` and the **RLC context** after full blob is retrieved (rsema1d.VerificationContext).
5. Reconstruct with rsema1d when enough rows gathered to reach `originalLength`.
6. **Optional**: reconstruct “zoda proof” as extra verification (§11.2).

### 5.3 Refresh() — **PFB resubmission only**

* Input: `commitment` (no data).
* Action: submit **new PFB** to extend/renew storage TTL.
* Servers’ **Promotion Listener** observes the PFB and **extends TTL** (without any row re-upload).
* Output: new PFB block height.
* Use cases: initial PFB confirmed late; proactive TTL extension before expiry.

---

## 6) Wire APIs (protobuf)

### 6.1 Bulk API (v1 default)

```proto
syntax = "proto3"; package fibre.v1;

message GF128 { bytes coeffs = 1; }          // len==16
message Commitment { bytes value = 1; }      // len==32
message RowWithProof { uint32 index = 1; bytes row = 2; bytes proof = 3; } // row len==2048
message RlcOrig { repeated GF128 coeffs = 1; }

message StorageAttestation {
  Commitment commitment = 1;
  string chain_id = 2;
  uint64 service_epoch_minute = 3; // UTC minutes
  bytes signature = 4;             // ed25519
}

message UploadRowsRequest {
  Commitment commitment = 1;
  RlcOrig rlc_orig = 2;
  repeated RowWithProof rows = 3;     // per-FSP subset
  optional uint64 original_length = 10; // advisory/metrics
}

message UploadRowsResponse {
  repeated StorageAttestation attestations = 1;
  bool deduplicated = 2;                // true if server already had this commitment
  optional uint32 backoff_ms = 10;      // server-advised cooldown
}

message GetRowsRequest {
  Commitment commitment = 1;
  optional bytes bitmap = 2;            // little-endian bits; bit i -> row i requested
  repeated uint32 indices = 3;          // alternative to bitmap
}

message GetRowsResponse {
  repeated RowWithProof rows = 1;       // subset present
  repeated uint32 missing_indices = 2;  // for partial fulfillment
}

service Fibre {
  rpc UploadRows(UploadRowsRequest) returns (UploadRowsResponse);
  rpc GetRows(GetRowsRequest) returns (GetRowsResponse);
}
```

**Why bulk?** For v1, per-FSP payload ≈ **300 KiB**, under typical gRPC limits; simpler retries and dedup.

### 6.2 Streaming API (kept; switch if payloads grow)

```proto
syntax = "proto3"; package fibre.v1;

message Init    { bytes commitment = 1; uint32 total_chunks = 2; uint64 total_bytes = 3; }
message Chunk   { uint32 index = 1; bytes data = 2; } // ≤ 2048 bytes each
message Finish  { bytes sha256 = 1; }
message UploadReq { oneof kind { Init init = 1; Chunk chunk = 2; Finish end = 3; } }

message UploadResp {
  repeated StorageAttestation attestations = 1;
  bool dedup = 2;
}

service ProofService { rpc Upload(stream UploadReq) returns (UploadResp); }
```

**When to use**: if `rows_per_val * row_size` grows well beyond a few hundred KiB, or infra enforces small message limits, or you want progressive verification/backpressure.

---

## 7) Attestations (sign/verify)

**Preimage**

```
sign_bytes = sha256(
  "FIBRE/commitment/v1" ||
  commitment ||
  chain_id ||
  service_epoch_minute_be64
)
```

* Domain separation prevents cross‑protocol replay.
* `chain_id` ties to network; `service_epoch_minute` binds to the **expiry minute** (UTC, floor of `expire_at/60`).

**Server**

* After persisting rows: `expire_at = now + unconfirmed_ttl + safety_buffer`; compute `service_epoch_minute`; **sign and return**.
* On promotion (PFB): may issue a **new attestation** for `now + confirmed_ttl`.

**Client**

* Rebuild preimage; verify **ed25519** against validator consensus key from ValTracker.
* Optional freshness check: current minute ≤ `service_epoch_minute`.

---

## 8) Server architecture

### 8.1 Components

* **Store**: **Badger** (v1). Alternatives **deferred** (SQL-light, epoch-buckets). (§8.7)
* **Fibre gRPC**: implements Bulk + Streaming.
* **Promotion Listener**: watches PFBs; calls `Promote()`; may return new attestation.
* **Rate limiter**: enforce throughput cap and fairness.

### 8.2 Badger layout

Keys:

* `d/<commitment>` → **blob** (big value; no TTL)
* `p/<commitment>` → pointer: `"u:<YYYYMMDDHHmm>"` or `"c:<YYYYMMDDHHmm>"`
* `bu/<YYYYMMDDHHmm>/<commitment>` → index for unconfirmed bucket
* `bc/<YYYYMMDDHHmm>/<commitment>` → index for confirmed bucket

Why this shape:

- **Promotion is tiny**: just move the pointer and swap a small bucket entry; the 300 KB `d/*` value is **not** rewritten.
- **GC is cheap**: at each minute boundary, iterate `bu/<nowmin>/*` and `bc/<nowmin>/*` and delete those items + their `d/*` + `p/*` keys.

**Reasons**: promotion updates only tiny keys; GC is a minute-bucket sweep; no big-value rewrite on promotion.

**Store API**

```go
type Store interface {
  Put(ctx context.Context, key [32]byte, value []byte, safety time.Duration) error
  Promote(ctx context.Context, key [32]byte) (promoted bool, err error)
  Get(ctx context.Context, key [32]byte) ([]byte, bool, error)
  RunGC(ctx context.Context)
  Close() error
}
```

**GC**: every minute boundary, sweep `bu/<now>/*` and `bc/<now>/*`, deleting bucket entries, pointer, and data; run Badger value-log GC opportunistically.

### 8.3 Upload verification pipeline (mandatory)

1. **Dedup** by `commitment` (if already present → `deduplicated=true`, return extant attestation if desired).
2. Build **rsema1d verification context** from `rlc_orig`.
3. For each row:

    * Verify **Merkle proof** vs `commitment`.
    * Verify **encoding soundness** using Context‑Based Verification with the RLC context.
    * **Why RLC check**: detects adversarial encodings that pass Merkle but are off‑codeword.
4. Persist rows with `expire_at = now + unconfirmed_ttl + safety_buffer` (batch write + sync).
5. Compute `service_epoch_minute`; **sign** domain‑tagged bytes; return **attestation(s)**.

### 8.4 Get pathway

* Accept `bitmap` or `indices`. If neither provided, return **all** stored rows for the commitment.
* Partial fulfillment allowed; include `missing_indices`.

### 8.5 Promotion Listener

* Subscribe to on-chain **PFB** events.
* On observed `commitment`, call `Promote()` to move from `bu/*` to `bc/*` and update pointer `p/*`.
* Optionally **issue a new attestation** for the confirmed period and expose via `GetAttestation(Commitment)` or piggyback on a promotion ack.

### 8.6 API limits & fairness

* Enforce `server_rows_per_message_limit` and maximum serialized request size.
* **Rate-limit** ingress to **\~10 MiB/s** (configurable). With \~**300 KiB**/req worst-case, this is **\~33 RPS**. This is **conservative** because not all requests are max size; start here and raise with telemetry.
* Include `backoff_ms` in responses to guide clients under pressure.

### 8.7 Storage roadmap

* **SQL deletes are not relevant in v1**. We **start with Badger**. Later we may prototype **SQL-light** and **epoch-bucket** stores and pick based on measured throughput and GC performance.

### 8.8 Operational notes

* Use Badger write batches; ACK only after `Flush()+Sync`.
* Value‑log GC after sweeps or periodically until no‑rewrite.
* Health endpoints: `/healthz` (process up), `/readyz` (DB open, GC loop running), metrics endpoint (Prometheus).

---

## 9) Security considerations

* **Attestation domain** prevents cross‑protocol replay; `chain_id` prevents cross‑network reuse; `service_epoch_minute` binds to a freshness window.
* **Consensus keys** (ed25519) are the identity; we do not bind endpoint/IP in v1. If needed later, add a nonce or endpoint hash in v2.
* **Row verification** includes Context‑Based Verification which is necessary to prevent malleability via valid‑looking but non‑decodable rows.

---

## 10) Observability

**Client metrics**: encode latency, rows count, bytes per FSP, upload latency per FSP, attestation verify failures, quorum time, PFB submit latency/height, refresh PFB count.

**Server metrics**: RPS, bytes/s, dedup hit rate, proof verify failures, RLC failures, write batch size, GC sweep durations, promotion count, `backoff_ms` emissions, Badger vlog GC stats.

**Logs**: structured; include commitment (hex32), validator addr, service\_epoch\_minute, request sizes, error codes.

---

## 11) Open questions & options

### 11.1 Replication factor & mapping

Let `K` = originals, `ρ` = parity ratio (`encoding_factor`), `R` = replication factor, `V` = validators.

Target rows per validator:

```
rows_per_val = ceil( (K * (1 + ρ) * R) / V )
```

Assignment options:

* **Deterministic R**: for each row, take top‑`R` lowest SipHash scores over validators.
* **Probabilistic**: choose a cutoff so expected replicas ≈ `R`.

Impact: quorum guarantees under `f < V/3` faulty validators; pick `R` to ensure reconstruction margin.

### 11.2 Optional extra verification on read

* Reconstruct **zoda proof** (if specified) and verify after row‑level checks; off by default; enable in high‑assurance mode.

### 11.3 Attestation lifecycle policy

* Should servers **always** return a fresh attestation on promotion? **Preferred** for simplicity of receipts; clients then store the longest‑lived receipt.

---

## 12) Testing plan

**Unit**: ShardMap determinism; RLC context verification; attestation digest vectors; bitmap parsing.

**Integration**: end‑to‑end Put/Get/Refresh with N mock FSPs; dedup behavior; promotion on synthetic PFB events; failure injection (missing rows, wrong proofs).

**Load**: sustain ≥60 MiB/s writes across 200 concurrent Puts (matches 200 RPS × 300 KiB) with batching and rate‑limit engaged; verify GC keeps DB bounded.

**Crash safety**: SIGKILL between batch `Flush()` and `Sync()`; ensure ACKs only after sync.

**Compatibility**: bulk ↔ streaming parity; client library mode vs light‑node mode; Active vs Lazy ValTracker.

---

## 13) Reference helpers (non‑normative)

### 13.1 Attestation helpers (Go)

```go
const domain = "FIBRE/commitment/v1"
func Digest(commitment [32]byte, chainID string, serviceEpochMinute uint64) [32]byte { /* sha256(domain||commitment||chainID||be64(min)) */ }
func VerifyEd25519(pub ed25519.PublicKey, sig []byte, d [32]byte) bool { /* ... */ }
```

### 13.2 ShardMap (SipHash idea)

```go
score := SipHash(commitment, append(valPubKey, u32be(rowIndex)...))
// Per validator: pick rows with lowest scores until rows_per_val reached
```

### 13.3 Request bitmap

* Little‑endian bit order; bit `i` → request row `i`.
* Servers may cap #bits set per request.

---

## 14) Lifecycle diagrams (ASCII)

### 14.1 Upload & promotion

```
Client                 FSP (many)                 Chain
  |  encode+valset  |                             |
  |---------------->|                             |
  |  UploadRows     |                             |
  |  (bulk 300 KiB) |-- verify (proof+RLC) -->    |
  |                 |-- persist -> sign attn -->  |
  |<----- attns ----|                             |
  |  2/3 quorum     |                             |
  |---- PFB (commitment, original_length) ------->|
  |                 |<------ event (PFB) ---------|
  |                 |-- Promote (24h TTL) --------|
```

### 14.2 Refresh (no re-upload)

```
Client                  Chain                 FSP (Promotion Listener)
  |--- Refresh PFB ---> |                     |
  |                     | ---- event (PFB) -->|
  |                     |                     |-- Promote (extend TTL)
  |<-- height --------- |                     |
```

---

## 15) Bulk vs Streaming trade‑off (explicit)

* For **v1** payloads, each validator’s share is **≈300 KiB**, so **Bulk** is **reasonable and simpler**.
* If payloads grow (bigger blobs, higher `rows_per_val`, higher `R`), **switch to Streaming** (`Init → Chunk → Finish`).
* Both APIs **coexist**; server may signal preferred mode via error code or `backoff_ms`/advice.

---

## 16) Configuration (suggested defaults)

```toml
[row]
size = 2048
max_rows = 65536
encoding_factor = 1

[client]
send_workers = 20
read_workers = 20
mode = "library"   # or "light-node"
valtracker = "lazy" # or "active"

[server]
unconfirmed_ttl = "5m"
safety_buffer   = "1m"
confirmed_ttl   = "24h"
rows_per_message_limit = 151
throughput_cap_bytes_per_sec = 10485760  # 10 MiB/s
```