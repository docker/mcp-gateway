# Changelog

## v1.7.1 (2026-08-15)

- Decode: quoted-key/array-value round-trip (SPEC 4.2).

## v1.7.0 (2026-08-14)

- **Numeric domain (spec v3.5.3, SPEC 2.3.2).** Specifies the canonical numeric domain as signed `int64` for integers and IEEE-754 double for non-integers. Earlier versions left integers beyond the double-exact range (2^53) to the host numeric type; this version parses integer literals to an exact `int64` on decode and on the JSON-to-value bridge, returns an out-of-range error for a value outside `int64` on both decode and encode, and models larger values (unsigned-64 identifiers, exact decimals) as strings. Canonical number formatting aligns to the domain: a double at or above 2^53 renders in exponent notation. Verified against new `numbers/017-024` and `errors-v2/041-042` conformance fixtures and the cross-SDK differential fuzz. `EncodeGeneric` keeps its `string` signature and panics on an out-of-domain host integer (loud rejection: no wire emitted, no substitution); `EncodeGenericChecked` returns the error instead. Keeping the signature avoids a Go module major and its permanent `/v2` import-path change for consumers.

## v1.6.2 (2026-08-09)

- **Spec v3.5.1 conformance (SPEC 5, score-rounding errata).** SPEC 5 now pins the graph `score` two-decimal wire form to round-half-to-even on the exact IEEE-754 double, resolving a midpoint divergence in the JavaScript and Kotlin SDKs. This SDK's `strconv.FormatFloat` / `fmt %.2f` formatter already rounds half-to-even, so there is no behavior change; re-verified against the new `graph-encode/004_score_midpoint_rounding` conformance fixture.

## v1.6.1 (2026-08-07)

- Decoders now reject a declared `[N]` section count that does not match the actual item count, in both directions, per SPEC Section 13 (Count Validation). A declared count smaller than the rows or entries present was previously read as a limit and the surplus was dropped; it is now an error. Covers the generic tabular, keyed-map, and root-array forms, the delta and full-set decoders, and the graph `## edges [N]` section. Valid payloads and encoding are unchanged.

## v1.6.0 (2026-08-07)

### Added
- Keyed-tabular map encoding (SPEC 7.2a): a JSON object whose values are all objects forming a tabular set is encoded as a keyed table (`## [N:]{key,...}`) - the shared value fields are declared once, with one key-prefixed row per member. Canonical by default, supported in nested and streaming positions, and integrated with generic delta using the map key as the identity.

### Changed
- Negative zero is canonicalized to `0` for both integer and floating-point values (SPEC 2.3.1).
- Canonical-output alignment across all six SDKs: object key ordering, graph header fields, and symbol ordering follow the specification and reference implementation exactly.

### Testing
- Conformance runners assert re-encode idempotence (`encode(decode(x)) == x`) for the generic, graph, and delta profiles; a differential cross-SDK fuzz was added to the verification suite.

## v1.5.1 (2026-07-18)

### Fixes

- `EncodeGeneric` now normalizes native Go container types passed directly by the caller (e.g. `[]map[string]any`, `map[string]int`, `[]SomeStruct`) at every nesting depth, not just at the root. The input normalizer previously recursed only through its reflection fallback; the fast paths for `map[string]any` and `[]any` returned the value without descending into it, so a nested field such as `[]map[string]any` (a distinct type from `[]any` in Go, which has no slice covariance) fell through the encoder's type switch to the default scalar path and emitted Go's `fmt` map printing instead of a tabular section. Routing the same value through `ParseJSONOrdered` was already correct because it produces fully canonical `*OrderedMap`/`[]any`. Pinned by `native_types_test.go` (#2).
- Go-specific: the other five SDKs were checked and are not affected. They dispatch on dynamic types (TypeScript, Python), type-erased generics (Kotlin), covariant array casts (Swift, verified: `[[String: Any]] as? [Any]` succeeds), or a pre-normalized value enum (Rust). None distinguish `[]map` from `[]any` the way Go's static slice types do.

## v1.5.0 (2026-07-12)

### Fixes

- Fixed: `EncodeDelta` emitted a header missing the mandatory `profile=graph` discriminator (SPEC 3.1/16.1); it is now `GCF profile=graph tool=...`. The conformance runner now hard-fails on unhandled operations and exercises the graph delta path end to end.
- Added `DecodeDelta` (parse a `GCF profile=graph delta=true` wire back into removed/added symbols and edge changes) and wired it to `VerifyDelta` for atomic apply against a base snapshot plus `pack_root` recomputation, rejecting a wrong `new_root` with `root_mismatch` (SPEC 10.4). `EncodeDelta`'s `## added` line now carries the trailing `distance` field (SPEC 3.4.1, Section 10.1), which `pack_root` hashes, so a consumer can reconstruct the new snapshot and verify it. The shared `graph-delta` fixtures now run end to end: 001 (encode), 002 (verified apply), 003 (`root_mismatch` rejection).
- Buffered graph encoder: order edges by source ID, then target ID, then edge type (SPEC 16.1), instead of emitting them in input order. Decode-invariant (edges are a set) and does not affect `pack_root` (which sorts edge records independently), so no content addresses change. Pinned by shared fixture `graph-encode/003`. Streaming edges remain in producer-arrival order.
- Streaming graph trailer: `distance_N` group counts (distance >= 3) are now emitted in group-header emission order, not Go map-iteration order, so the trailer is deterministic (SPEC 16.1). Previously multiple `distance_N` groups produced a randomly-ordered trailer across runs. The encoder now records group-header emission order and builds the trailer from it.
- Decoder: reject an orphan `.field` attachment (a `.field` whose name is neither a `^`-marked column of its row nor a `>`-containing field name, SPEC 7.4.6.1.4) instead of silently absorbing it as an undeclared extra field. Such a stray attachment previously decoded to a record no encoder produces, silently injecting a field onto the last-parsed row (a lossless round-trip hole); now rejected per SPEC 16.5 (`orphan_attachment`).
- Decoder: reject an orphan positional inline body (a pipe-delimited line with no eligible `^{}` attachment-marker cell) instead of silently dropping it. The object-body parser previously skipped any unrecognized line, so a stray positional body (e.g. a second `Bob|b@t.com` after a row's one inline cell was filled) vanished with no error (silent data loss); now rejected per SPEC 16.5 (`orphan_inline_attachment`).
- Graph streaming trailer: the edge count is now always the last `counts` entry, even when the stream has no edges (positional `counts=2,1,0`; labeled `counts=…,edges:0`). A zero-edge stream previously dropped it, violating the SPEC §8.4 / §8.4.1 rule that the edge count is always present and last (the invariant that keeps the positional form unambiguous). The graph trailer is decoder-ignored, so this changes producer output only.

### Streaming: opt-in labeled trailer counts (SPEC §8.4.1)

- New `StreamOptions.LabeledTrailerCounts`. When set, the `##! summary` graph streaming trailer emits `counts=` in the labeled form `label:count` per group (e.g. `counts=targets:2,related:1,edges:3`) instead of the default positional values-only form (`counts=2,1,3`). Default false is byte-identical to prior output.
- Opt-in and non-breaking: a producer-side comprehension aid for known weak consumers. The trailer counts remain informational (decoder-ignored) in both forms; neither changes the decoded payload.
- Conformance: the `graph-stream-encode` runner reads the fixture `options.labeledTrailerCounts` and drives the shared fixture 005 (labeled) alongside 004 (positional).

## v1.4.1 (2026-07-12)

### Fixes

- **Streaming graph header now carries `profile=graph`.** The streaming encoder emitted a bare `GCF tool=...` header, omitting the REQUIRED profile discriminator (SPEC §3.1, §16.1). This was a Go-only divergence from the buffered encoder and the other five SDKs, and a strict conforming decoder would reject it. The streaming header is now `GCF profile=graph tool=...`, matching every other encode path. The wire trailer was already correct (`##! summary ... counts=`).

### Conformance and validation

- Graph `Decode` now requires `profile=graph` as the first header field (SPEC §16.3), rejecting a missing or mismatched profile.
- New `graph-stream-encode` conformance operation and shared fixture that drive the streaming encoder and compare its exact header and trailer bytes. Streaming encode previously had only decode fixtures, which is how the header regression escaped.
- Unit tests for the streaming header and for decoder profile strictness.

### Docs

- README streaming example: corrected the trailer from the defunct `## _summary ... sections=...` to the real `##! summary ... counts=...`.

## v1.4.0 (2026-07-12)

### Generic-profile delta encoding (SPEC §10a)

- Full producer + consumer implementation of generic-profile delta:
  - `GenericSet` (keyed record set), `GenericDeltaPayload`
  - `GenericPackRoot` (`gcf-pack-root-v1`, generic profile) with a purpose-built cell canonicalization decoupled from the wire cell encoder — collision-free (null/bool/number bare, strings always quoted) and record-safe
  - `DiffGenericSets` (the blessed producer path; centralizes the keyed-diff invariants), `EncodeGenericFull`, `EncodeGenericDelta`
  - `DecodeGenericFull`, `DecodeGenericDelta` (consumer wire parsing)
  - `VerifyGenericDelta` (atomic apply + `new_root` verification)
- Delta is opt-in and bilateral; the existing `EncodeGeneric` path is unchanged (backward compatible).

### Re-anchor session helper (producer convenience, SPEC §10a.8)

- `GenericDeltaSession` — a thin, stateful producer helper over the primitives that manages the re-anchor cadence: each `Next(next)` emits either a compact delta or, on its chosen cadence, a full re-anchor (the spec's "full" outcome), updating its held base. It introduces **no new wire syntax** (every payload is exactly what `EncodeGenericFull`/`EncodeGenericDelta` produce) and the cadence knobs are never wire fields — the wire spec stays cadence-agnostic.
- Pluggable policy: `FixedN(n)` (re-anchor every `n` turns; default `DefaultReanchorN = 15`) or `SizeGuard()` (re-anchor once the cumulative delta since the last anchor reaches the current full payload's byte size — the size-adaptive, production-recommended mode). A schema change forces a full (§10a.7).
- `CurrentFull()` returns the base as a full payload (send first / manual re-anchor); `Next` returns `(wire, isFull, err)`.

### Tests

- Self-proving round-trip (diff -> encode -> apply -> recomputed root), determinism, no-type-collision, every invariant/error path, full-payload wire round-trip, and the complete server -> wire -> consumer end-to-end loop.
- Decoder-robustness suite (malformed/truncated wire fails closed, never panics) and two fuzz targets (`FuzzGenericDeltaDecode`: decoder never panics; `FuzzGenericStringRoundTrip`: arbitrary UTF-8 string cells round-trip preserving the pack root).
- Conformance runner support for `generic-pack-root`, `generic-delta`, `generic-delta-verify`, `generic-delta-decode`, and `generic-delta-session` (15 shared fixtures).
- Session tests: FixedN cadence pattern, SizeGuard trigger, schema-change forces full, and the load-bearing "consumer applies every emission and stays byte-for-byte in sync with the producer at each turn" loop under both policies.

## v1.3.2 (2026-07-10)

### Fixes

- **Losslessness (nested null):** a nested object that is null at an intermediate level (e.g. `{"meta":{"owner":null}}`) is no longer flattened. Previously its leaves encoded as absent (`~`) and unflattened to a missing key, silently dropping the null. Such fields now fall back to the attachment mechanism; a top-level null still flattens losslessly (emits `-`, reconstructs via the all-null rule). Enforced by the shared conformance fixtures `flatten/017`–`019`. Prototype pollution does not affect Go (maps have no mutable prototype).

### Tests

- `TestPropertyRoundTripFlatten`: aligned arrays whose shared fields are fixed-shape nested objects, with a field or an intermediate nested level sometimes null/absent — the shape the prior scalar-only generator never produced, leaving the flatten/unflatten path unexercised. Verified to fail on the pre-fix encoder and pass on the fix.

## v1.3.1 (2026-06-23)

### Flatten Opt-Out

- Added `GenericOptions` struct with `NoFlatten` field to disable nested object flattening
- `EncodeGeneric(data, gcf.GenericOptions{NoFlatten: true})` produces attachment syntax instead of path columns
- Backward compatible: `EncodeGeneric(data)` behavior unchanged (flatten on by default)
- CLI: `gcf encode-generic --no-flatten` flag
- Fuzz testing covers both flatten-on and flatten-off paths
- Fixed: field names containing `>` no longer appear as tabular columns (spec rule 7.4.6.1.4)
- Fixed: field names containing `>` no longer eligible for flattening analysis
- Fixed: decoder no longer treats literal `>` in key names as a path separator
- Fixed: decoder accepts orphan attachments (fields excluded from column list)
- Fuzz key generator now includes `>` for adversarial testing

## v1.3.0 (2026-06-22)

### Spec v3.2: Nested Object Flattening

- Encoder automatically flattens fixed-shape nested objects into `>` path column names (e.g., `"customer>name"` instead of `^` + `.customer {}` attachment)
- Decoder reconstructs nested objects from `>` path columns
- 20-48% fewer tokens on deeply nested API data (Jira, Stripe, K8s, calendar events)
- 100% comprehension on every frontier model (validated across 9 models, 7 providers)
- Zero regression on lossless round-trips (200K random + 100K adversarial + 10M bracket-colon)
- Falls back to attachment mechanism for: variable-length arrays, objects with different keys across rows, objects with `>` in key names, empty nested objects
- All-absent leaves (`~`) omit the parent key; all-null leaves (`-`) set parent to null

## v1.2.0 (2026-06-14)

### Spec v3.1

- `tool` field in graph profile header is now optional (SHOULD be present for MCP, not required)

### Bug Fixes

- Quote strings containing commas (conformance: `inline-schema/006_inline_with_quoted_values`)
- Decode v2-format indented attachments in tabular rows (conformance: `decode/002_attachment`)
- Full v3 conformance: 157/157 fixtures passing, 200M+ round-trips verified

## v1.1.0 (2026-06-12)

### Breaking Changes

- `EncodeGeneric` now produces inline schema format (not backwards compatible with v1.x decoders)
- Attachment lines no longer indented (same depth as parent row)
- Inline object fields use positional encoding without field-name prefix

### New Features

- Inline object schema: objects with 3+ scalar fields encoded positionally with `^{fields}` header
- Shared array schemas: identical nested arrays omit `{fields}` after first row
- 472M+ fuzz iterations across all 6 implementations, zero failures

### Bug Fixes

- Quote strings starting with `.` (dot prefix)
- Quote C1 control characters (U+0080-U+009F)
- Quote Unicode whitespace (NBSP, hair space, etc.)

## v1.0.2 (2026-06-10)

- CLI: `encode-generic` and `decode-generic` subcommands for generic profile
- CLI now supports both graph and generic profiles

## v1.0.0 (2026-06-10)

Reference implementation for GCF SPEC v2.0. All 133 conformance fixtures passing. 20M property-based round-trips with zero failures. 7.9M fuzz executions with zero crashes.

### Breaking changes

- `EncodeGeneric` emits `GCF profile=generic` header on every payload
- `DecodeGeneric` requires `GCF profile=generic` or `GCF profile=graph` header
- Strings colliding with typed literals are now quoted (`"true"`, `"123"`, `"-"`)
- Full JSON string escaping (`\b`, `\f`, `\n`, `\r`, `\t`, `\uXXXX`, surrogate pairs)
- Full JSON number grammar with exponent notation and canonical formatting
- Null is `-`, absent field in tabular rows is `~`
- Nested values in tabular rows use `^` marker with `.field {}` / `.field [N]` attachments
- Expanded arrays use explicit type markers: `@N =scalar`, `@N {}`, `@N [M]`
- Root scalars: `=value`; root arrays: anonymous `## [N]`
- Streaming trailer changed from `## _summary` to `##! summary counts=N,M,...`
- Graph encoder emits `profile=graph` in header
- Graph encoder sorts symbols by score descending within distance groups
- Graph encoder assigns IDs after sorting (sequential in output order)
- Session encoder uses session-stable IDs across calls

### New

- `scalar.go`: common scalar grammar (quoting, escaping, parsing, number formatting)
- `orderedmap.go`: `OrderedMap` type preserving JSON key insertion order
- `ParseJSONOrdered`: ordered JSON parser for conformance-grade encoding
- Duplicate key rejection in decoder
- Tab/indent validation in decoder
- Orphan attachment detection in decoder
- Item ID validation in expanded arrays
- `##!` summary count validation (arity and value)
- Graph decoder returns v2.0 error categories
- Graph encoder sorts symbols by score descending, assigns IDs after sorting
- Delta section validation in graph decoder
- 133-fixture conformance test runner (`conformance_v2_test.go`)
- Property-based round-trip tests: 10M random + 10M adversarial values (`roundtrip_test.go`)
- Fuzz targets for encoder and decoder (`fuzz_test.go`); found and fixed 3 bugs:
  - Negative zero lost during int64 conversion
  - Large integer precision loss outside float64 exact range (2^53)
  - Quoted `}` in field declarations misidentified as closing brace
- Delta section validation in graph decoder
- 131-fixture conformance test runner (`conformance_v2_test.go`)

## v0.6.0 (2026-06-06)

- `DecodeGeneric`: decode any GCF text (tabular or graph) back to Go values
- `GenericStreamEncoder`: zero-buffering tabular streaming encode (BeginArray/WriteRow/EndArray/WriteKV/WriteSection/WriteInlineArray)

## v0.5.0 (2026-06-06)

- `NewStreamEncoder`: zero-buffering streaming encode to any `io.Writer`
- `WriteSymbol`, `WriteEdge`, `WriteBareRef`: emit lines immediately as data arrives
- `Close`: emits `## _summary` trailer with final counts
- O(1) memory per row, thread-safe
- Decoder handles `[?]` deferred counts and `## _summary` (no changes needed)

## v0.4.0 (2026-06-05)

- `EncodeGeneric`: primitive arrays inlined as `name[N]: val1,val2,val3`
- Eliminates TOON's only benchmark win (deeply nested config)

## v0.3.0 (2026-06-05)

- **Breaking**: `Encode()` now emits `edges=N` in header line
- **Breaking**: `Encode()` now emits `## edges [N]` section header (was `## edges`)
- `Decode()` updated to parse `## edges [N]` format (strips bracket suffix)
- Session encoder updated to emit new edge count format
- Comprehension eval expanded to 13 questions, achieves 13/13 with new format

## v0.1.2 (2026-06-04)

- Fix: decoder rejects headers missing required `tool` field (conformance)

## v0.1.1 (2026-06-03)

- `EncodeGeneric`: encode arbitrary Go values (maps, slices, structs) into GCF tabular format
- Tabular encoding: positional rows with pipe separators, section headers, nested field support
- Uniform array detection with 70% key overlap threshold

## v0.2.0 (2026-06-03)

- 3-way comprehension eval (GCF vs TOON vs JSON at 500 symbols)
- Eval moved to isolated submodule (`eval/go.mod`) to avoid polluting root deps
- Results: GCF 100% accuracy at 21% of JSON's token cost

## v0.1.0 (2026-06-03)

- Initial release
- `Encode` / `Decode`: full GCF round-trip
- `EncodeWithSession`: session deduplication (92.7% savings by 5th call)
- `EncodeDelta`: delta encoding for re-queries (81.2% savings)
- Thread-safe `Session` type
- 16 kind abbreviations
- Full test suite
