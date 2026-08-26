package gcf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// OrderedMap is a JSON object that preserves key insertion order.
// Used as the intermediate representation for conformance-grade encoding.
type OrderedMap struct {
	keys   []string
	values map[string]any
}

// NewOrderedMap creates an empty OrderedMap.
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{values: make(map[string]any)}
}

// Set adds or updates a key-value pair, preserving insertion order for new keys.
func (m *OrderedMap) Set(key string, value any) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Get retrieves a value by key.
func (m *OrderedMap) Get(key string) (any, bool) {
	v, ok := m.values[key]
	return v, ok
}

// Keys returns keys in insertion order.
func (m *OrderedMap) Keys() []string {
	return m.keys
}

// Len returns the number of entries.
func (m *OrderedMap) Len() int {
	return len(m.keys)
}

// ToMap converts to a plain map (loses order).
func (m *OrderedMap) ToMap() map[string]any {
	out := make(map[string]any, len(m.keys))
	for _, k := range m.keys {
		v := m.values[k]
		switch val := v.(type) {
		case *OrderedMap:
			out[k] = val.ToMap()
		case []any:
			out[k] = orderedSliceToPlain(val)
		default:
			out[k] = v
		}
	}
	return out
}

func orderedSliceToPlain(arr []any) []any {
	out := make([]any, len(arr))
	for i, v := range arr {
		switch val := v.(type) {
		case *OrderedMap:
			out[i] = val.ToMap()
		case []any:
			out[i] = orderedSliceToPlain(val)
		default:
			out[i] = v
		}
	}
	return out
}

// MarshalJSON implements json.Marshaler, preserving key order.
func (m *OrderedMap) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range m.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		keyJSON, _ := json.Marshal(k)
		b.Write(keyJSON)
		b.WriteByte(':')
		valJSON, err := json.Marshal(m.values[k])
		if err != nil {
			return nil, err
		}
		b.Write(valJSON)
	}
	b.WriteByte('}')
	return []byte(b.String()), nil
}

// ParseJSONOrdered parses JSON bytes into Go values using OrderedMap for objects.
// Preserves key insertion order, which map[string]any does not.
func ParseJSONOrdered(data []byte) (any, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	return parseJSONValue(dec)
}

func parseJSONValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			return parseJSONObject(dec)
		case '[':
			return parseJSONArray(dec)
		default:
			return nil, fmt.Errorf("unexpected delimiter: %v", v)
		}
	case json.Number:
		// Token shape follows domain (SPEC 2.3.2): a bare-integer literal (no
		// fraction, no exponent) is an int64-domain integer and MUST NOT be routed
		// through float64, which would silently approximate magnitudes beyond 2^53.
		// A decimal or exponent literal is a double.
		s := v.String()
		if !strings.ContainsAny(s, ".eE") {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				if errors.Is(err, strconv.ErrRange) {
					return nil, fmt.Errorf("out_of_range: integer %s is outside the canonical int64 domain [-9223372036854775808, 9223372036854775807]; model larger values as strings (SPEC 2.3.2)", s)
				}
				return nil, err
			}
			return n, nil
		}
		f, err := v.Float64()
		if err != nil {
			return nil, err
		}
		return f, nil
	case string:
		return v, nil
	case bool:
		return v, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected token type: %T", tok)
	}
}

func parseJSONObject(dec *json.Decoder) (*OrderedMap, error) {
	m := NewOrderedMap()
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if delim, ok := tok.(json.Delim); ok && delim == '}' {
			return m, nil
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %T", tok)
		}
		val, err := parseJSONValue(dec)
		if err != nil {
			return nil, err
		}
		m.Set(key, val)
	}
}

func parseJSONArray(dec *json.Decoder) ([]any, error) {
	var arr []any
	for {
		if !dec.More() {
			// Consume closing bracket.
			dec.Token()
			return arr, nil
		}
		val, err := parseJSONValue(dec)
		if err != nil {
			if err == io.EOF {
				return arr, nil
			}
			return nil, err
		}
		arr = append(arr, val)
	}
}
