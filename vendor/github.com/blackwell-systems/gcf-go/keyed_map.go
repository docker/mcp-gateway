package gcf

import (
	"fmt"
	"strings"
)

// keyedMapEligible reports whether an object (map[string]any or *OrderedMap) is
// a keyed map of objects that should render as a keyed table `## [N:]{key,...}`
// (SPEC 7.2a, prototype). It returns the ordered member keys, the corresponding
// value objects, the ordered value-field union, and the key-column label.
func keyedMapEligible(m any, opts encodeOpts) (keys []string, values []any, valueFields []string, keyLabel string, ok bool) {
	switch mm := m.(type) {
	case map[string]any:
		if len(mm) == 0 {
			return nil, nil, nil, "", false
		}
		keys = orderedKeys(mm)
		for _, k := range keys {
			values = append(values, mm[k])
		}
	case *OrderedMap:
		if mm.Len() == 0 {
			return nil, nil, nil, "", false
		}
		keys = mm.Keys()
		for _, k := range keys {
			v, _ := mm.Get(k)
			values = append(values, v)
		}
	default:
		return nil, nil, nil, "", false
	}

	// A keyed map requires at least two members: the form factors the shared
	// value fields into one header, which only pays off across multiple members.
	// A single-member map yields a one-row table the same size as a section, so
	// keying it would change canonical output for every nested single-member
	// object (e.g. `{"data":{...}}` wrappers) with no benefit. Single-member
	// objects use ordinary encoding; a single-key wrapper of a multi-member map
	// therefore defers, and the inner map is keyed at its own level (SPEC 7.2a.1).
	if len(keys) < 2 {
		return nil, nil, nil, "", false
	}

	// Every value must be an object; build the ordered field union.
	seen := make(map[string]bool)
	for _, v := range values {
		var vk []string
		switch vo := v.(type) {
		case map[string]any:
			vk = orderedKeys(vo)
		case *OrderedMap:
			vk = vo.Keys()
		default:
			return nil, nil, nil, "", false // non-object value
		}
		for _, f := range vk {
			if !seen[f] {
				seen[f] = true
				valueFields = append(valueFields, f)
			}
		}
	}
	if len(valueFields) == 0 {
		return nil, nil, nil, "", false // all-empty value objects
	}

	// A keyed header needs at least one value field that can be a tabular column.
	// A field name containing ">" cannot be a column (SPEC 7.4.6.1.4); if every
	// value field contains ">", the keyed form would have only the key column,
	// which is invalid. Such a map uses Section 7.2 section encoding instead, the
	// object analogue of an array falling back to expanded form.
	hasColumn := false
	for _, f := range valueFields {
		if !strings.Contains(f, ">") {
			hasColumn = true
			break
		}
	}
	if !hasColumn {
		return nil, nil, nil, "", false
	}

	// Key-column label: "key", made unique by prepending "_" on collision.
	keyLabel = "key"
	inUnion := func(s string) bool {
		for _, f := range valueFields {
			if f == s {
				return true
			}
		}
		return false
	}
	for inUnion(keyLabel) {
		keyLabel = "_" + keyLabel
	}
	return keys, values, valueFields, keyLabel, true
}

// encodeKeyedMap emits a keyed table for a map of objects. It augments each
// value object with the key column and routes through encodeTabular with the
// keyed bracket, so nested-value handling (flatten/inline/attachment/null/
// absent) is inherited unchanged. name is empty for a root/anonymous map.
func encodeKeyedMap(b *strings.Builder, name string, named bool, keys []string, values []any, valueFields []string, keyLabel string, depth int, opts encodeOpts) {
	encodeKeyedMapWithPrefix(b, keyedHeaderPrefix(name, named, depth), keys, values, valueFields, keyLabel, depth, opts)
}

// keyedHeaderPrefix builds the header prefix. named distinguishes an anonymous
// root keyed map (`## `) from a named member whose name may itself be the empty
// string (`## ""`), which formatKey quotes so it round-trips as a distinct level
// rather than collapsing into the anonymous root form.
func keyedHeaderPrefix(name string, named bool, depth int) string {
	prefix := indentStr(depth)
	if !named {
		return prefix + "## "
	}
	return prefix + "## " + formatKey(name) + " "
}

// encodeKeyedMapWithPrefix emits `<headerPrefix>[N:]{...}` and the keyed rows,
// reusing encodeTabular. headerPrefix is the full prefix up to the count bracket.
func encodeKeyedMapWithPrefix(b *strings.Builder, headerPrefix string, keys []string, values []any, valueFields []string, keyLabel string, depth int, opts encodeOpts) {
	fields := make([]string, 0, len(valueFields)+1)
	fields = append(fields, keyLabel)
	fields = append(fields, valueFields...)

	arr := make([]any, len(keys))
	for i, k := range keys {
		aug := make(map[string]any, len(valueFields)+1)
		switch vo := values[i].(type) {
		case map[string]any:
			for kk, vv := range vo {
				aug[kk] = vv
			}
		case *OrderedMap:
			for _, kk := range vo.Keys() {
				vv, _ := vo.Get(kk)
				aug[kk] = vv
			}
		}
		aug[keyLabel] = k
		arr[i] = aug
	}
	encodeTabular(b, headerPrefix, arr, fields, depth, opts, true)
}

// keyedRowsToMap reconstructs the map from decoded keyed-table rows: the first
// declared field is the member key; the remaining fields form the value object.
// Members are emitted in row order and each value object keeps the row's declared
// field order (with the key column removed).
func keyedRowsToMap(rows []any, fields []string) (*OrderedMap, error) {
	if len(fields) < 2 {
		return nil, fmt.Errorf("keyed_map: header must declare at least two fields")
	}
	keyLabel := fields[0]
	out := NewOrderedMap()
	for _, r := range rows {
		row, ok := r.(*OrderedMap)
		if !ok {
			return nil, fmt.Errorf("keyed_map: row is not an object")
		}
		kv, present := row.Get(keyLabel)
		if !present {
			return nil, fmt.Errorf("keyed_map: row missing key column %q", keyLabel)
		}
		ks, ok := kv.(string)
		if !ok {
			ks = fmt.Sprintf("%v", kv)
		}
		if _, dup := out.Get(ks); dup {
			return nil, fmt.Errorf("keyed_map: duplicate member key %q", ks)
		}
		// Rebuild the member value in declared order minus the key column.
		val := NewOrderedMap()
		for _, k := range row.Keys() {
			if k == keyLabel {
				continue
			}
			v, _ := row.Get(k)
			val.Set(k, v)
		}
		out.Set(ks, val)
	}
	return out, nil
}
