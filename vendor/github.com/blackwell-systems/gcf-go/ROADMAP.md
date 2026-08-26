# Roadmap

## v3.1 changes applied

- [x] `tool` field made optional in graph profile header (was required, now SHOULD for MCP)
- [x] Conformance fixture `graph-decode/003_no_tool_field.json` passes
- [x] Error fixture `errors-v2/022_missing_graph_tool.json` removed from spec repo (no longer an error)

## Conformance: 157/157 passing

- [x] `decode/002_attachment.json`: Fixed v2 indented attachment parsing in `decode_generic.go`
- [x] `inline-schema/006_inline_with_quoted_values.json`: Added comma to quoting rules in `scalar.go`
