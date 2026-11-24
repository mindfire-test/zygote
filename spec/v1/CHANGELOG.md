# Changelog — spec/v1

All notable changes to the `zygote` specification format version 1 will be documented in this file.

The specification format is versioned independently of the Go reference implementation under Semantic Versioning (SRS NFR-6.3).

---

## [1.0.0] - 2026-11-16

### Added
- Initial formalized specification of the `spec/v1` format.
- Canonical binary tree encoding specification (§2) with BLAKE3-256 root hashing and empty tree constant `ec2bd03bf86b935fa34d71ad7ebb049f1f10f87d343e521511d8f9e6625620cd`.
- Canonical JSON Manifest schema specification per RFC 8785 (§3).
- Bundle archive `.zip` directory layout and verification rules (§4).
- JSON-RPC 2.0 stdio Harness protocol specification (§5).
- Divergence taxonomy and error classifications (§6).
- Language-agnostic conformance test fixtures and conformance test runner (`spec/conformance/`).
