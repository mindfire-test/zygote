# zygote Format Specification — v1

**Document Status:** Standard (M4)  
**Version:** 1.0.0  
**License:** CC-BY-4.0  
**Date:** 2026-11-16  

---

## 1. Overview

`zygote` (`zyg`) records what an AI agent did, replays it deterministically elsewhere, and enables forking from any checkpoint step.

This specification formally defines:
1. **Canonical Binary Tree Encoding** for content-addressed directory trees (§2)
2. **Canonical JSON Manifest Schema** per RFC 8785 for `trace.json` (§3)
3. **Bundle Container Layout** for portable `.zip` archives (§4)
4. **Harness Protocol** (JSON-RPC 2.0 over line-delimited `stdio`) (§5)
5. **Divergence Taxonomy & Verification Semantics** (§6)
6. **Compatibility & Conformance Criteria** (§7)

Any third-party tool adhering to this specification can read, verify, replay, and generate valid Zygote artifacts without relying on the Go reference implementation.

---

## 2. Canonical Binary Tree Encoding

A world state filesystem is structured as a Merkle tree of content-addressed nodes.
- **Blobs** are raw file content stored by their cryptographic digest.
- **Trees** represent directory nodes containing zero or more directory entries.

### 2.1 Cryptographic Hash Function
- **Algorithm:** **BLAKE3-256** (`lukechampine.com/blake3`), producing a fixed 32-byte hash value.
- **Empty Tree Root:**
  The empty tree (`Tree{}`) has an established constant hash value (the BLAKE3 hash of 4 zero bytes `0x00000000`):
  ```
  ec2bd03bf86b935fa34d71ad7ebb049f1f10f87d343e521511d8f9e6625620cd
  ```
  The empty tree is never serialized to the object store; creating an empty world requires zero I/O.

### 2.2 Entry Structure
Each entry in a tree represents either a regular file (`KindFile = 0`) or a subdirectory (`KindDir = 1`):

| Field | Type | Encoding | Description |
|---|---|---|---|
| `name_len` | `uint16` | Big-endian unsigned 16-bit integer | Byte length of `name` |
| `name` | `[]byte` | UTF-8 bytes (length `name_len`) | Filename/directory entry name |
| `kind` | `uint8` | Byte (`0x01` = file, `0x02` = directory) | Type of entry |
| `mode` | `uint32` | Big-endian unsigned 32-bit integer | POSIX permission bits (`0o777` mask) |
| `size` | `uint64` | Big-endian unsigned 64-bit integer | File size in bytes (0 for directories) |
| `hash` | `[32]byte` | 32 raw bytes | BLAKE3 hash of content blob or child tree |

### 2.3 Tree Serialization Rules
A tree byte sequence is serialized with strictly deterministic ordering and layout:
1. **Header:** 4 bytes (`uint32` big-endian) representing `entry_count`.
2. **Entries:** Concatenation of encoded entries, ordered strictly in **ascending lexicographical order by entry `name`** (`bytes.Compare(a.Name, b.Name) < 0`).
3. **Strict Path Rules:**
   - Entry names must never contain `/` path separators.
   - Entry names `.` and `..` are strictly prohibited.
   - Symbolic links, sockets, fifos, and non-regular files are never followed or recorded.
   - No timestamps (mtime, ctime, atime) or system-specific metadata are preserved.

### 2.4 Hashing Tree Objects
The 32-byte hash of a tree is:
```
hash = BLAKE3-256(encoded_tree_bytes)
```
Because serialization is strictly canonical, identical directory contents always produce identical tree hashes regardless of file creation order or underlying filesystem.

---

## 3. Canonical JSON Manifest Schema (RFC 8785)

The manifest is written to `trace.json` within each run recording and bundle.

### 3.1 JSON Schema
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "ZygoteRecordingManifest",
  "type": "object",
  "required": [
    "version",
    "hash_algorithm",
    "producer",
    "has_raw_prompts",
    "steps",
    "effects"
  ],
  "properties": {
    "version": {
      "type": "integer",
      "enum": [1]
    },
    "hash_algorithm": {
      "type": "string",
      "enum": ["blake3", "sha256"]
    },
    "producer": {
      "type": "string",
      "description": "Identifier of writer tool or agent, e.g. zyg/v0.2.0"
    },
    "redaction_applied": {
      "type": "string",
      "description": "Optional name or digest of redaction policy applied"
    },
    "fidelity": {
      "type": "object",
      "required": ["tier"],
      "properties": {
        "tier": {
          "type": "string",
          "enum": ["A", "B", "C"]
        },
        "limitations": {
          "type": "array",
          "items": { "type": "string" }
        }
      }
    },
    "has_raw_prompts": {
      "type": "boolean",
      "description": "True if unredacted model inputs are retained"
    },
    "meta": {
      "type": "object",
      "additionalProperties": { "type": "string" }
    },
    "steps": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["n", "name", "root", "effects"],
        "properties": {
          "n": { "type": "integer", "minimum": 0 },
          "name": { "type": "string" },
          "root": {
            "type": "string",
            "pattern": "^[0-9a-f]{64}$",
            "description": "64-character lowercase hexadecimal BLAKE3 root hash"
          },
          "effects": { "type": "integer", "minimum": 0 },
          "metadata": {
            "type": "object",
            "additionalProperties": { "type": "string" }
          }
        }
      }
    },
    "effects": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["seq", "op", "key", "value"],
        "properties": {
          "seq": { "type": "integer", "minimum": 0 },
          "op": { "type": "string" },
          "key": { "type": "string" },
          "value": {
            "type": "string",
            "description": "Standard Base64-encoded byte payload"
          }
        }
      }
    }
  }
}
```

### 3.2 Canonicalization (RFC 8785)
Before computing manifest digests or applying digital signatures:
- JSON keys must be lexicographically sorted by Unicode code point.
- Whitespace between tokens is eliminated (no formatting spaces or newlines).
- Numbers are formatted without trailing zeroes or exponential notation where integer.
- String characters are escaped per RFC 8785 specification rules.

---

## 4. Bundle Container Layout

A Zygote bundle is a self-contained archive packaged using standard PKZIP 2.0 format (`.zip`).

### 4.1 Directory Structure
```
run.zip
├── trace.json
├── objects/
│   ├── <64-hex-blake3-hash-1>
│   ├── <64-hex-blake3-hash-2>
│   └── ...
```

### 4.2 Bundle Verification Rules
1. **Path Traversal Protection (NFR-4.1, NFR-4.3):**
   - Every entry path must start with either `trace.json` or `objects/<64-char-hex>`.
   - File names containing `..`, absolute paths starting with `/` or `\`, or entries writing outside the target world root must cause immediate rejection.
2. **Object Integrity Verification (FR-4.3):**
   - For every object in `objects/<hex>`, the uncompressed object payload must be hashed with BLAKE3-256.
   - If `BLAKE3(payload) != hex`, the bundle must be rejected as corrupt (`ErrHashMismatch`).
3. **Step Reachability Verification (FR-4.4):**
   - For every step in `steps[]`, its `root` hash must be verified.
   - The root tree object must exist in `objects/`.
   - All subtrees and referenced file blobs must be recursively verified. Any missing referenced object must cause immediate rejection (`ErrMissingObject`).

---

## 5. Harness Protocol Specification (IF-3)

The Harness protocol enables any external programming language (Python, TypeScript, Rust, etc.) to participate in record and replay without network ports or specialized daemon dependencies.

### 5.1 Transport
- **Framing:** Line-delimited JSON-RPC 2.0 over standard I/O (`stdin` / `stdout`).
- **Parent Process:** Zygote (`zyg record` or `zyg replay`).
- **Child Process:** Agent program.
- **Log Demuxing:** Any line emitted on `stdout` by the child that cannot be parsed as valid JSON-RPC 2.0 is forwarded directly to `stderr` as agent diagnostic logging, preserving protocol integrity.

### 5.2 JSON-RPC 2.0 Methods

#### 1. `handshake`
Negotiates protocol version at the start of the execution.
- **Params:** `{"version": 1}`
- **Result:** `{"version": 1}`
- **Semantics:** If versions do not match or cannot be negotiated, execution fails closed.

#### 2. `effect`
Interception boundary for non-deterministic external calls (LLM completions, random generation, clocks, HTTP calls, tools).
- **Params:**
  ```json
  {
    "op": "model",
    "key": "prompt-hash-or-id",
    "value": "base64-bytes-on-record"
  }
  ```
- **Result:**
  ```json
  {
    "value": "base64-bytes-returned"
  }
  ```
- **Semantics:**
  - **On Record:** Agent invokes external effect, sends value to harness. Harness appends to journal and returns value.
  - **On Replay:** Agent calls harness with `op` and `key`. Harness checks next sequential entry. If matching, harness returns recorded `value` (the external function is **never invoked**). If mismatched or exhausted, harness aborts and triggers divergence.

#### 3. `step`
Marks a logical checkpoint boundary and compares or captures world state.
- **Params:**
  ```json
  {
    "name": "plan_phase"
  }
  ```
- **Result:**
  ```json
  {
    "match": true,
    "expected": "hex-snapshot-hash",
    "actual": "hex-snapshot-hash"
  }
  ```
- **Semantics:**
  - **On Record:** Captures current filesystem world hash and step metadata.
  - **On Replay:** Verifies actual replayed world hash against recorded step root.

---

## 6. Divergence Taxonomy & Semantics

When replaying a run, Zygote checks both the effect sequence and world state snapshots. Any deviation is reported at the exact first step of divergence with one of the following canonical classifications:

| Divergence Class | Code | Meaning | Common Cause |
|---|---|---|---|
| `effect-mismatch` | `effect-mismatch` | Different `op` or `key` requested at position | Prompt template or tool parameters modified |
| `world-mismatch` | `world-mismatch` | Identical effects, but resulting world hash differs | Agent code or business logic modified |
| `step-mismatch` | `step-mismatch` | Next step name differs from recording | Agent control flow or loop logic changed |
| `exhausted` | `exhausted` | Agent requested more effects than recorded | Agent is performing additional work |
| `missing-object` | `missing-object` | Required blob or tree missing from bundle | Truncated, tampered, or over-redacted bundle |
| `uninstrumented` | `uninstrumented` | Divergence with no corresponding code change | Uninstrumented source of non-determinism |

---

## 7. Conformance Requirements

A conforming implementation of Zygote:
1. Must correctly decode and encode canonical trees per §2 and match the reference BLAKE3 hash vectors in `spec/conformance/trees/`.
2. Must validate bundles per §4, accepting valid bundles and rejecting corrupt, truncated, or traversal bundles matching fixtures in `spec/conformance/bundles/`.
3. Must reproduce the exact divergence classification verdicts on replaying recordings in `spec/conformance/recordings/`.
4. Must pass all test vectors in `spec/conformance/` without error.
