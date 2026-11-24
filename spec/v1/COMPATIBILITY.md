# Compatibility Policy — spec/v1

**Version:** 1.0  
**License:** CC-BY-4.0  

---

## 1. Scope & Guarantee

The `spec/v1` format provides forward and backward compatibility guarantees for all consumers, runtimes, and adapters:

1. **Additive Changes (Minor/Patch):**
   - New optional fields may be introduced into the JSON manifest (`trace.json`) or harness protocol frames.
   - Any compliant parser encountering unrecognized optional fields must preserve or ignore them without failing.
2. **Breaking Changes (Major v2):**
   - Required fields will never be removed, renamed, or have their types altered in `spec/v1`.
   - The canonical binary tree encoding format (§2) and BLAKE3 hashing algorithm are frozen for `v1`.
   - Any incompatible modification requires a major version increment to `spec/v2`.
3. **Deprecation & Transition Window:**
   - Any compliant implementation must support `spec/v1` bundles and recordings for at least **two minor releases** following any publication of a subsequent specification version.
