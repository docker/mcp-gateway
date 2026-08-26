<p align="center">
  <a href="https://gcformat.com/playground.html"><img src="https://img.shields.io/badge/playground-live-2563eb?style=for-the-badge" alt="Playground"></a>
  <a href="https://gcformat.com/guide/benchmarks.html"><img src="https://img.shields.io/badge/benchmarks-2%2C500%2B%20evals-22c55e?style=for-the-badge" alt="Benchmarks"></a>
  <a href="https://pkg.go.dev/github.com/blackwell-systems/gcf-go"><img src="https://img.shields.io/badge/pkg.go.dev-reference-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="pkg.go.dev"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-333?style=for-the-badge" alt="License"></a>
</p>

<p align="center">
  <img src="assets/gcf-hero-wire-delta.png" alt="gcf-go" width="760">
</p>

# gcf-go

Go implementation of [GCF](https://gcformat.com/), the most token-efficient wire format for LLMs. A drop-in alternative to JSON and TOON for any structured data.

<p align="center">
  <img src="assets/divider-wave-2.png" alt="" width="100%">
</p>

<p align="center">
  <img src="assets/social-preview.png" alt="gcf-go" width="80%">
</p>

<p align="center">
  <img src="assets/divider.png" alt="" width="100%">
</p>

**Built for the agentic loop, where the same structured context crosses the model boundary turn after turn.** A single payload is 50-92% smaller than JSON, but GCF also deduplicates repeated structure across turns and sends only deltas when context changes, so by the 5th overlapping call each response costs 99% fewer tokens than JSON, and a 10-call session runs 94.4% cheaper than re-sending JSON every turn. Session dedup and delta both need local IDs and a multi-turn design that neither JSON nor TOON has.

- **100% comprehension on every frontier model**, zero training. 29% fewer tokens than TOON and 56% fewer than JSON across 16 datasets; 91.2% on structurally complex code graphs (vs TOON 68.8%, JSON 54.1%).
- **Proven lossless** across 43,000,000,000+ round-trips in 5 formats and 6 languages. Zero runtime dependencies.
- **One format, four properties no other single format holds at once:** schema-free, lossless, token-compact (50-92% vs JSON), and model-readable with zero training. JSON is verbose, Protobuf needs a schema, MessagePack is binary, and TOON isn't reliably lossless.

2,500+ LLM evaluations. [Full benchmarks](https://gcformat.com/guide/benchmarks.html).

Docs: [gcformat.com](https://gcformat.com/) · [Playground](https://gcformat.com/playground.html) · [GCF vs TOON](https://gcformat.com/guide/vs-toon.html)

## Install

```
go get github.com/blackwell-systems/gcf-go
```

Zero dependencies. Single package. Don't want to change code? Use the [MCP proxy](https://github.com/blackwell-systems/gcf-proxy) for zero-code adoption.

## CLI

Standalone binaries are attached to each [release](https://github.com/blackwell-systems/gcf-go/releases). The CLI is optional; it's for converting files from the command line without writing code.

```bash
# Install
go install github.com/blackwell-systems/gcf-go/cmd/gcf@latest

# Or download a binary from the latest release

# Usage
gcf encode < payload.json    # JSON to GCF
gcf decode < payload.gcf     # GCF to JSON
gcf stats  < payload.json    # token comparison
```

## Quick Start

```go
import gcf "github.com/blackwell-systems/gcf-go"

data := map[string]any{
    "employees": []map[string]any{
        {"id": 1, "name": "Alice", "department": "Engineering", "salary": 95000},
        {"id": 2, "name": "Bob", "department": "Sales", "salary": 72000},
    },
}
output := gcf.EncodeGeneric(data)
```

Output:
```
## employees [2]{department,id,name,salary}
Engineering|1|Alice|95000
Sales|2|Bob|72000
```

Works on any Go value: maps, slices, structs. One header declares field names, rows are positional values.

## Graph Profile

For code graph data with symbols, edges, and distance groups:

```go
p := &gcf.Payload{
    Tool: "context_for_task", TokenBudget: 5000, TokensUsed: 1847,
    Symbols: []gcf.Symbol{
        {QualifiedName: "pkg.Auth", Kind: "function", Score: 0.78, Provenance: "lsp", Distance: 0},
        {QualifiedName: "pkg.Server", Kind: "function", Score: 0.54, Provenance: "lsp", Distance: 1},
    },
    Edges: []gcf.Edge{{Source: "pkg.Server", Target: "pkg.Auth", EdgeType: "calls"}},
}
output := gcf.Encode(p)
```

Output:
```
GCF tool=context_for_task budget=5000 tokens=1847 symbols=2 edges=1
## targets
@0 fn pkg.Auth 0.78 lsp
## related
@1 fn pkg.Server 0.54 lsp
## edges [1]
@0<@1 calls
```

## Decode

```go
p, err := gcf.Decode(input)
if err != nil {
    log.Fatal(err)
}
fmt.Println(p.Tool, len(p.Symbols), "symbols", len(p.Edges), "edges")
```

## Session Deduplication

Track transmitted symbols across multiple tool responses. Previously-sent symbols become bare references instead of full declarations:

```go
sess := gcf.NewSession()

out1 := gcf.EncodeWithSession(payload1, sess) // full declarations
out2 := gcf.EncodeWithSession(payload2, sess) // reused symbols as "@N  # previously transmitted"
```

By the 5th call in a session: 86% fewer tokens than JSON from dedup alone, 99% stacked with delta encoding.

## Streaming Encode

Write GCF output incrementally as symbols and edges arrive. Zero buffering, O(1) memory per row. Ideal for MCP servers that walk large graphs or paginate results:

```go
enc := gcf.NewStreamEncoder(w, "context_for_task", gcf.StreamOptions{TokenBudget: 5000})

// Symbols emit immediately as they're discovered.
enc.WriteSymbol(gcf.Symbol{QualifiedName: "pkg.Auth", Kind: "function", Score: 0.95, Provenance: "lsp", Distance: 0})
enc.WriteSymbol(gcf.Symbol{QualifiedName: "pkg.Server", Kind: "function", Score: 0.60, Provenance: "lsp", Distance: 1})

// Edges emit immediately too.
enc.WriteEdge(gcf.Edge{Source: "pkg.Server", Target: "pkg.Auth", EdgeType: "calls"})

// Close emits the ##! summary trailer with final counts.
enc.Close()
```

Output:
```
GCF tool=context_for_task budget=5000
## targets
@0 fn pkg.Auth 0.95 lsp
## related
@1 fn pkg.Server 0.60 lsp
## edges [?]
@0<@1 calls
##! summary symbols=2 edges=1 counts=1,1,1
```

The `[?]` marker signals deferred count. The `##! summary` trailer provides counts after the data. The LLM has both the data and the counts in context. Standard `Decode()` handles streaming output with no changes.

## Delta Encoding

When the consumer already has a prior context pack, send only what changed:

```go
delta := &gcf.DeltaPayload{
    Tool:     "context_for_task",
    BaseRoot: "aaa111",
    NewRoot:  "bbb222",
    Removed:  []gcf.Symbol{{QualifiedName: "pkg.OldFunc", Kind: "function"}},
    Added:    []gcf.Symbol{{QualifiedName: "pkg.NewFunc", Kind: "function", Score: 0.85, Provenance: "rwr"}},
    DeltaTokens: 30,
    FullTokens:  200,
}

output := gcf.EncodeDelta(delta)
```

81.2% savings on re-queries where the pack changed slightly.

## Generic Encoding

Encode any Go value (not just graph payloads) into GCF tabular format:

```go
data := map[string]any{
    "employees": []map[string]any{
        {"id": 1, "name": "Alice", "department": "Engineering", "salary": 95000},
        {"id": 2, "name": "Bob", "department": "Sales", "salary": 72000},
    },
}
output := gcf.EncodeGeneric(data)
```

Output:
```
## employees [2]{id,name,department,salary}
1|Alice|Engineering|95000
2|Bob|Sales|72000
```

Works on maps, slices, structs, and primitives. Arrays of uniform objects get tabular rows. Nested objects use `## key` section headers.

## API

| Function | Description |
|----------|-------------|
| `Encode(p *Payload) string` | Encode a graph payload to GCF text |
| `EncodeGeneric(data any) string` | Encode any value to GCF tabular format |
| `Decode(input string) (*Payload, error)` | Parse GCF text back to a Payload |
| `EncodeWithSession(p *Payload, s *Session) string` | Encode with session deduplication |
| `EncodeDelta(d *DeltaPayload) string` | Encode a delta (added/removed only) |
| `NewStreamEncoder(w, tool, opts) *StreamEncoder` | Create a streaming encoder (zero-buffering) |
| `NewSession() *Session` | Create a new session tracker (thread-safe) |

## Types

| Type | Purpose |
|------|---------|
| `Payload` | Full GCF payload: tool, budget, symbols, edges, pack root |
| `Symbol` | Graph node: qualified name, kind, score, provenance, distance |
| `Edge` | Directed relationship: source, target, edge type |
| `DeltaPayload` | Diff between two packs: added/removed symbols and edges |
| `Session` | Thread-safe tracker for multi-call deduplication |
| `StreamEncoder` | Streaming encoder: WriteSymbol, WriteEdge, WriteBareRef, Close |
| `StreamOptions` | Config for streaming: TokenBudget, TokensUsed, PackRoot, Session |
| `KindAbbrev` / `KindExpand` | Bidirectional kind abbreviation maps |

## Benchmarks

2,500+ LLM evaluations across 11 models, 4 providers, and 50+ independent test runs.

| | GCF | TOON | JSON |
|---|---|---|---|
| **Comprehension** (23 runs, 10 models) | **91.2%** | 68.8% | 54.1% |
| **Generation** (28 runs, 9 models) | **5/5** | 1.0/5 | 5.0/5 |
| **Input tokens** (500 symbols) | **11,090** | 16,378 | 53,341 |
| **Output tokens** (100 symbols) | **5,976** | 8,937 | 16,121 |

GCF wins 15/16 datasets on the expanded [token efficiency benchmark](https://github.com/blackwell-systems/toon/tree/gcf-comparison). Full results: [gcformat.com/guide/benchmarks](https://gcformat.com/guide/benchmarks.html)

## Implementations

| Language | Package | Repository |
|----------|---------|-----------|
| Go | `go get github.com/blackwell-systems/gcf-go` | [gcf-go](https://github.com/blackwell-systems/gcf-go) |
| TypeScript | `npm install @blackwell-systems/gcf` | [gcf-typescript](https://github.com/blackwell-systems/gcf-typescript) |
| Python | `pip install gcf-python` | [gcf-python](https://github.com/blackwell-systems/gcf-python) |
| Rust | `cargo add gcf` | [gcf-rust](https://github.com/blackwell-systems/gcf-rust) |
| Swift | Swift Package Manager | [gcf-swift](https://github.com/blackwell-systems/gcf-swift) |
| Kotlin | JitPack | [gcf-kotlin](https://github.com/blackwell-systems/gcf-kotlin) |
| MCP Proxy | `pip install gcf-proxy` | [gcf-proxy](https://github.com/blackwell-systems/gcf-proxy) (bidirectional, session dedup, HTTP frontend) |
| Claude Code Plugin | `/plugin install` | [gcf-claude-plugin](https://github.com/blackwell-systems/gcf-claude-plugin) (one-command install, session stats hook) |
| Codex Plugin | `codex plugin add` | [gcf-codex-plugin](https://github.com/blackwell-systems/gcf-codex-plugin) (one-command install, session stats hook) |
| VS Code | `ext install blackwell-systems.gcf-vscode` | [gcf-vscode](https://marketplace.visualstudio.com/items?itemName=blackwell-systems.gcf-vscode) (syntax highlighting) |
| n8n | `npm install n8n-nodes-gcf` | [gcf-n8n-nodes](https://github.com/blackwell-systems/gcf-n8n-nodes) (workflow encode/decode) |
| Tree-sitter | `npm install tree-sitter-gcf` | [tree-sitter-gcf](https://github.com/blackwell-systems/tree-sitter-gcf) |

**Zero runtime dependencies. Permanently.** All six implementations depend only on their language's standard library. No transitive dependencies. No supply chain risk. This is a permanent commitment: GCF will never take on external runtime dependencies. MIT licensed. All implementations support both generic profile (`encodeGeneric`) and graph profile (`encode`). CLI included in all 6 languages.

**Specification:** [SPEC v3.5.3 Stable](https://github.com/blackwell-systems/gcf/blob/main/SPEC.md) with 279 conformance fixtures, 43,000,000,000+ lossless round-trips verified across 5 formats and 6 languages. Current versions: Go v1.7.0, TypeScript v2.6.0, Python v2.6.0, Rust v3.0.0, Swift v2.7.0, Kotlin v2.6.0, .NET v0.2.0. Cross-language conformance verified across all seven SDKs.

## Adopted by

| Project | |
|---------|--|
| **[Chrome DevTools MCP](https://github.com/ChromeDevTools/chrome-devtools-mcp)** | 47K★ · the Google Chrome DevTools team's MCP server; exposes live browser state (DOM, network, console, performance) to AI coding agents |
| **[Speakeasy](https://speakeasy.com)** | OpenAPI tooling (customers include Google, Verizon, Mistral AI, DocuSign, Vercel); GCF is a native output format in their `oq` CLI |
| **[OmniRoute](https://omniroute.online)** | 17K★ · AI gateway, registry, and proxy between AI clients and model providers; GCF vendored into its compression engine |
| **[NetClaw](https://github.com/automateyournetwork/netclaw)** | 610★ · AI-powered network automation (113 skills, 66 MCP integrations); replaced TOON with GCF across every MCP server |
| **[ctx](https://github.com/stevesolun/ctx)** | 552★ · real-time context selector for Claude Code; surfaces only the relevant tools from a 103K-node knowledge graph |
| **[Lynkr](https://github.com/Fast-Editor/Lynkr)** | 531★ · local LLM gateway for AI coding clients; GCF as a drop-in tool-result compressor alongside TOON |
| **[Open Data Products SDK](https://opendataproducts.org/sdk/)** | Linux Foundation · Python toolkit and MCP server for data-product standards; GCF sidecars for agent context |
| **[NeuroNest](https://neuronest.cc)** | agent-first IDE; first commercial GCF adoption, across four encoding surfaces with session dedup and delta |
| **[Raycast](https://raycast.com/blackwell-systems/json-to-gcf-converter)** | JSON-to-GCF Converter extension in the Raycast Store, for the macOS productivity launcher |

[See all adopters →](https://gcformat.com/ecosystem/adopters.html)

## License

MIT - [Dayna Blackwell](https://github.com/blackwell-systems)
