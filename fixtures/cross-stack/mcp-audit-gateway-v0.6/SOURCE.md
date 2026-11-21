# Source Provenance

These vectors are extracted from the mcp-audit-gateway project's conformance
test suite. They exercise the tuple-array canonicalization format and
hash-chained checkpoint mechanism used for tamper-evident audit records.

## Upstream

- Repository: https://github.com/elang2/mcp-audit-gateway
- Tag: v0.6.0
- Commit: 5564da9
- Tag SHA: 69afbaba40f2da3b0d97420f04067eac897959b6

## File Hashes (SHA-256)

```
34f8261aacb666c4bff9e48a2fe7cbda6647a3fb295d371a1a7e8bd5e3826a32  canonicalization.json
1eadd73cef1910c91e911eb57a496bc8e4c373c9c33820233c8b654714877d70  checkpoint.json
```

## What These Vectors Test

**canonicalization.json** covers the tuple-array canonical form used for
signing audit records. It includes field ordering, null handling, conditional
fields (decisionContextDigest, parties, extensionsDigest), the injective
type-tagged canonicalizeValue() algorithm (M/L tags, safe-integer constraint,
UTF-16 key sorting), and negative cases (floats throw, lone surrogates throw).

**checkpoint.json** covers the checkpoint and chain-integrity mechanism.
It includes basic checkpoint canonicalization, chain continuity across log
rotation boundaries, truncation detection via externalized checkpoints,
sequence regression detection, chain_break records, and verification modes
(strict vs relative/suffix).

## Relevance to mcp-gateway

These vectors validate behavior needed by an audit attestation interceptor
(issue #557). The interceptor produces hash-chained records; consumers need
to verify chain integrity, detect truncation, and confirm checkpoints without
implementing the canonicalization from scratch. These vectors let any
implementation verify compatibility with the upstream format.

## Cross-SDK Context

The v0.6.0 release also includes cross-SDK differential testing that found
26 serialization divergences across all 10 official MCP SDKs. The
canonicalization format used here (tuple-array with type tags) was designed
specifically to be immune to these divergences: it uses safe integers only
(no float formatting variance), explicit field ordering (no key-sort
disagreements), and surrogate rejection (no encoding ambiguity).

See: https://github.com/elang2/mcp-audit-gateway/blob/v0.6.0/test/vectors/SDK-AUDIT.md

## License

Apache-2.0 (same as upstream repository)
