package interceptors

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	gcf "github.com/blackwell-systems/gcf-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/log"
)

// GCFOutputMiddleware re-encodes JSON tool-call results as GCF (Graph Compact Format,
// https://gcformat.com) when the encoding is both smaller and losslessly reversible.
//
// It is opt-in (gateway --gcf-output) and strictly conservative: a result is rewritten
// only when it parses as JSON, the GCF form is shorter, and it round-trips to the exact
// same value. Anything else (non-JSON/prose text, a payload that does not shrink, or one
// that does not round-trip) is passed through unchanged, so tool output is never larger
// and never altered. StructuredContent and non-text content are left untouched.
func GCFOutputMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			response, err := next(ctx, method, req)
			if err != nil {
				return response, err
			}

			result, ok := response.(*mcp.CallToolResult)
			if !ok || result.IsError {
				return response, err
			}

			for _, content := range result.Content {
				text, ok := content.(*mcp.TextContent)
				if !ok {
					continue
				}
				if encoded, changed := gcfEncodeIfBeneficial(text.Text); changed {
					log.Logf("  > GCF: re-encoded tool result (%d -> %d bytes)\n", len(text.Text), len(encoded))
					text.Text = encoded
				}
			}

			return result, err
		}
	}
}

// gcfEncodeIfBeneficial returns the GCF encoding of a JSON text when it is strictly
// smaller and round-trips to the same value, and reports (original, false) otherwise.
func gcfEncodeIfBeneficial(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		// Only JSON objects/arrays can benefit; leave prose and scalars alone.
		return text, false
	}

	// ParseJSONOrdered preserves key order and decodes numbers exactly (json.Number),
	// so integers beyond 2^53 are never silently rounded through float64.
	parsed, err := gcf.ParseJSONOrdered([]byte(text))
	if err != nil {
		return text, false
	}

	// EncodeGenericChecked returns an error (never panics) for a number outside the
	// canonical int64 domain, so an out-of-range value simply falls back to JSON.
	wire, err := gcf.EncodeGenericChecked(parsed)
	if err != nil {
		return text, false
	}

	// Never grow: keep JSON unless GCF is strictly smaller.
	if len(wire) >= len(text) {
		return text, false
	}

	// Never corrupt: keep JSON unless the GCF form decodes back to the same value.
	roundTrip, err := gcf.DecodeGeneric(wire)
	if err != nil {
		return text, false
	}
	if !losslessEqual(parsed, roundTrip) {
		return text, false
	}

	return wire, true
}

// losslessEqual reports whether two decoded JSON values are semantically identical,
// independent of object key order and number representation. It is order-insensitive
// (map keys are sorted by json.Marshal) and number-exact (integers keep full int64
// precision instead of collapsing through float64).
func losslessEqual(a, b any) bool {
	aBytes, err1 := json.Marshal(normalizeForCompare(a))
	bBytes, err2 := json.Marshal(normalizeForCompare(b))
	return err1 == nil && err2 == nil && bytes.Equal(aBytes, bBytes)
}

// normalizeForCompare rewrites a decoded value into a canonical form for comparison:
// gcf.OrderedMap becomes a plain map (json.Marshal then sorts keys), and every number
// becomes a canonical json.Number so that 5, 5.0, int64(5) and json.Number("5") all
// compare equal while large integers keep exact precision.
func normalizeForCompare(v any) any {
	switch t := v.(type) {
	case *gcf.OrderedMap:
		m := make(map[string]any, t.Len())
		for _, k := range t.Keys() {
			val, _ := t.Get(k)
			m[k] = normalizeForCompare(val)
		}
		return m
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = normalizeForCompare(val)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, val := range t {
			s[i] = normalizeForCompare(val)
		}
		return s
	case json.Number:
		return canonicalNumber(t)
	case int64:
		return json.Number(strconv.FormatInt(t, 10))
	case int:
		return json.Number(strconv.FormatInt(int64(t), 10))
	case float64:
		return canonicalNumber(json.Number(strconv.FormatFloat(t, 'g', -1, 64)))
	default:
		return v
	}
}

// canonicalNumber collapses a json.Number to a canonical textual form: an exact int64
// where the value is an integer in range, otherwise the shortest float64 rendering.
func canonicalNumber(n json.Number) json.Number {
	if i, err := n.Int64(); err == nil {
		return json.Number(strconv.FormatInt(i, 10))
	}
	if f, err := n.Float64(); err == nil {
		if f == math.Trunc(f) && !math.IsInf(f, 0) && math.Abs(f) < 9.007199254740992e15 {
			return json.Number(strconv.FormatInt(int64(f), 10))
		}
		return json.Number(strconv.FormatFloat(f, 'g', -1, 64))
	}
	return n
}
