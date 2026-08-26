package gcf

import (
	"fmt"
	"strings"
)

// GenericOptions controls optional behavior of EncodeGeneric.
type GenericOptions struct {
	// NoFlatten disables promotion of fixed-shape nested objects to path
	// columns (e.g. "customer>name") in tabular sections. When true, nested
	// objects use attachment syntax instead. Open-weight models currently
	// comprehend the expanded form better; this gap is expected to close.
	NoFlatten bool
}

// EncodeGeneric encodes any Go value into the GCF generic profile with all v3
// optimizations: inline object schemas, no attachment indentation, no field prefix on
// inline attachments, shared array schemas, MinInlineFields = 3. Pass GenericOptions to
// customize behavior (e.g. disable flattening).
//
// It panics if the value contains an integer outside the canonical int64 numeric domain
// (SPEC 2.3.2) — in practice an unsigned 64-bit integer above math.MaxInt64. Typical
// inputs (int/int64/string/float, or anything decoded from JSON) never contain one, so
// this cannot trigger in normal use; the panic is loud rejection of an out-of-domain
// value, never a silent substitution. A caller that may encode untrusted native values
// with such integers should use EncodeGenericChecked to get the error instead.
func EncodeGeneric(data any, optsList ...GenericOptions) string {
	s, err := EncodeGenericChecked(data, optsList...)
	if err != nil {
		panic(err)
	}
	return s
}

// EncodeGenericChecked is EncodeGeneric with the numeric-domain error returned rather
// than panicked: it returns an out-of-range error for an integer outside int64 (SPEC
// 2.3.2) instead of approximating the value or substituting a string. Model larger
// values as strings at the producer.
func EncodeGenericChecked(data any, optsList ...GenericOptions) (string, error) {
	var gopts GenericOptions
	if len(optsList) > 0 {
		gopts = optsList[0]
	}
	opts := encodeOpts{
		InlineObjectSchema: true,
		DropAttachIndent:   true,
		DropFieldPrefix:    true,
		SharedArraySchema:  true,
		MinInlineFields:    3,
		FlattenNested:      !gopts.NoFlatten,
	}
	return encodeGenericImpl(data, opts)
}

// encodeOpts controls which optimizations are active.
type encodeOpts struct {
	InlineObjectSchema bool
	DropAttachIndent   bool
	DropFieldPrefix    bool
	SharedArraySchema  bool
	MinInlineFields    int
	FlattenNested      bool
}

func (o encodeOpts) String() string {
	var parts []string
	if o.InlineObjectSchema {
		parts = append(parts, "inline-obj")
	}
	if o.DropAttachIndent {
		parts = append(parts, "no-indent")
	}
	if o.DropFieldPrefix {
		parts = append(parts, "no-prefix")
	}
	if o.SharedArraySchema {
		parts = append(parts, "shared-arr")
	}
	if o.MinInlineFields > 0 {
		parts = append(parts, fmt.Sprintf("min%d", o.MinInlineFields))
	}
	if len(parts) == 0 {
		return "v2-baseline"
	}
	return strings.Join(parts, "+")
}

func encodeGenericImpl(data any, opts encodeOpts) (string, error) {
	// Normalize and enforce the numeric domain up front (SPEC 2.3.2). toAny errors on
	// an out-of-int64 integer, so once it succeeds the encode tree below sees only
	// in-domain values and stays infallible.
	v, err := toAny(data)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("GCF profile=generic\n")
	encodeRootValue(&b, v, opts)
	return b.String(), nil
}

func encodeRootValue(b *strings.Builder, v any, opts encodeOpts) {
	switch val := v.(type) {
	case nil:
		b.WriteString("=-\n")
	case *OrderedMap:
		if ks, vs, vf, kl, ok := keyedMapEligible(val, opts); ok {
			encodeKeyedMap(b, "", false, ks, vs, vf, kl, 0, opts)
			return
		}
		encodeOrderedObject(b, val, 0, opts)
	case map[string]any:
		if ks, vs, vf, kl, ok := keyedMapEligible(val, opts); ok {
			encodeKeyedMap(b, "", false, ks, vs, vf, kl, 0, opts)
			return
		}
		encodeObject(b, val, 0, opts)
	case []any:
		encodeRootArray(b, val, opts)
	default:
		b.WriteByte('=')
		b.WriteString(formatScalar(v, 0))
		b.WriteByte('\n')
	}
}

func encodeOrderedObject(b *strings.Builder, m *OrderedMap, depth int, opts encodeOpts) {
	prefix := indentStr(depth)
	for _, key := range m.Keys() {
		val, _ := m.Get(key)
		fk := formatKey(key)
		switch v := val.(type) {
		case *OrderedMap:
			if ks, vs, vf, kl, ok := keyedMapEligible(v, opts); ok {
				encodeKeyedMap(b, key, true, ks, vs, vf, kl, depth, opts)
				continue
			}
			b.WriteString(prefix)
			b.WriteString("## ")
			b.WriteString(fk)
			b.WriteByte('\n')
			encodeOrderedObject(b, v, depth+1, opts)
		case map[string]any:
			if ks, vs, vf, kl, ok := keyedMapEligible(v, opts); ok {
				encodeKeyedMap(b, key, true, ks, vs, vf, kl, depth, opts)
				continue
			}
			b.WriteString(prefix)
			b.WriteString("## ")
			b.WriteString(fk)
			b.WriteByte('\n')
			encodeObject(b, v, depth+1, opts)
		case []any:
			encodeNamedArray(b, fk, v, depth, opts)
		default:
			b.WriteString(prefix)
			b.WriteString(fk)
			b.WriteByte('=')
			b.WriteString(formatScalar(val, 0))
			b.WriteByte('\n')
		}
	}
}

func encodeObject(b *strings.Builder, m map[string]any, depth int, opts encodeOpts) {
	prefix := indentStr(depth)
	for _, key := range orderedKeys(m) {
		val := m[key]
		fk := formatKey(key)
		switch v := val.(type) {
		case *OrderedMap:
			if ks, vs, vf, kl, ok := keyedMapEligible(v, opts); ok {
				encodeKeyedMap(b, key, true, ks, vs, vf, kl, depth, opts)
				continue
			}
			b.WriteString(prefix)
			b.WriteString("## ")
			b.WriteString(fk)
			b.WriteByte('\n')
			encodeOrderedObject(b, v, depth+1, opts)
		case map[string]any:
			if ks, vs, vf, kl, ok := keyedMapEligible(v, opts); ok {
				encodeKeyedMap(b, key, true, ks, vs, vf, kl, depth, opts)
				continue
			}
			b.WriteString(prefix)
			b.WriteString("## ")
			b.WriteString(fk)
			b.WriteByte('\n')
			encodeObject(b, v, depth+1, opts)
		case []any:
			encodeNamedArray(b, fk, v, depth, opts)
		default:
			b.WriteString(prefix)
			b.WriteString(fk)
			b.WriteByte('=')
			b.WriteString(formatScalar(val, 0))
			b.WriteByte('\n')
		}
	}
}

func encodeRootArray(b *strings.Builder, arr []any, opts encodeOpts) {
	if len(arr) == 0 {
		b.WriteString("## [0]\n")
		return
	}
	if allPrimitives(arr) {
		parts := make([]string, len(arr))
		for i, v := range arr {
			parts[i] = formatScalar(v, ',')
		}
		fmt.Fprintf(b, "## [%d]: %s\n", len(arr), strings.Join(parts, ","))
		return
	}
	if fields := tabularFields(arr); fields != nil {
		encodeTabular(b, "## ", arr, fields, 0, opts, false)
		return
	}
	encodeExpanded(b, "## ", arr, 0, opts)
}

func encodeNamedArray(b *strings.Builder, name string, arr []any, depth int, opts encodeOpts) {
	prefix := indentStr(depth)
	if len(arr) == 0 {
		fmt.Fprintf(b, "%s## %s [0]\n", prefix, name)
		return
	}
	if allPrimitives(arr) {
		parts := make([]string, len(arr))
		for i, v := range arr {
			parts[i] = formatScalar(v, ',')
		}
		fmt.Fprintf(b, "%s%s[%d]: %s\n", prefix, name, len(arr), strings.Join(parts, ","))
		return
	}
	if fields := tabularFields(arr); fields != nil {
		encodeTabular(b, fmt.Sprintf("%s## %s ", prefix, name), arr, fields, depth, opts, false)
		return
	}
	encodeExpanded(b, fmt.Sprintf("%s## %s ", prefix, name), arr, depth, opts)
}

func encodeExpanded(b *strings.Builder, headerPrefix string, arr []any, depth int, opts encodeOpts) {
	prefix := indentStr(depth)
	fmt.Fprintf(b, "%s[%d]\n", headerPrefix, len(arr))
	for i, item := range arr {
		switch v := item.(type) {
		case *OrderedMap:
			if ks, vs, vf, kl, ok := keyedMapEligible(v, opts); ok {
				encodeKeyedMapWithPrefix(b, fmt.Sprintf("%s@%d ", prefix, i), ks, vs, vf, kl, depth+1, opts)
				continue
			}
			fmt.Fprintf(b, "%s@%d {}\n", prefix, i)
			encodeOrderedObject(b, v, depth+1, opts)
		case map[string]any:
			if ks, vs, vf, kl, ok := keyedMapEligible(v, opts); ok {
				encodeKeyedMapWithPrefix(b, fmt.Sprintf("%s@%d ", prefix, i), ks, vs, vf, kl, depth+1, opts)
				continue
			}
			fmt.Fprintf(b, "%s@%d {}\n", prefix, i)
			encodeObject(b, v, depth+1, opts)
		case []any:
			if len(v) == 0 {
				fmt.Fprintf(b, "%s@%d [0]\n", prefix, i)
			} else if allPrimitives(v) {
				parts := make([]string, len(v))
				for j, pv := range v {
					parts[j] = formatScalar(pv, ',')
				}
				fmt.Fprintf(b, "%s@%d [%d]: %s\n", prefix, i, len(v), strings.Join(parts, ","))
			} else if nf := tabularFields(v); nf != nil {
				encodeTabular(b, fmt.Sprintf("%s@%d ", prefix, i), v, nf, depth+1, opts, false)
			} else {
				encodeExpanded(b, fmt.Sprintf("%s@%d ", prefix, i), v, depth+1, opts)
			}
		default:
			fmt.Fprintf(b, "%s@%d =%s\n", prefix, i, formatScalar(item, 0))
		}
	}
}

// fieldAttachment extends fieldAttachment with inline schema info.
type fieldAttachment struct {
	name         string
	value        any
	inline       bool
	inlineFields []string
}

// inlineSchemaFields checks if a given field across all rows in a tabular array
// is eligible for inline schema encoding: all values are objects with the same
// keys and all values are primitives (no nested objects or arrays).
// The first row must have the field so the decoder sees ^{fields} on row 0.
func inlineSchemaFields(arr []any, fieldName string) []string {
	if len(arr) > 0 {
		v, exists := objectItemGet(arr[0], fieldName)
		if !exists || v == nil {
			return nil
		}
		if objectItemKeys(v) == nil {
			return nil
		}
	}
	var canonicalKeys []string
	for _, item := range arr {
		v, exists := objectItemGet(item, fieldName)
		if !exists || v == nil {
			continue
		}
		keys := objectItemKeys(v)
		if keys == nil {
			return nil
		}
		for _, k := range keys {
			val, _ := objectItemGet(v, k)
			switch val.(type) {
			case *OrderedMap, map[string]any, []any:
				return nil
			}
		}
		if canonicalKeys == nil {
			canonicalKeys = keys
		} else {
			if len(keys) != len(canonicalKeys) {
				return nil
			}
			for i, k := range keys {
				if k != canonicalKeys[i] {
					return nil
				}
			}
		}
	}
	return canonicalKeys
}

func inlineSchemaFieldsMin(arr []any, fieldName string, minFields int) []string {
	fields := inlineSchemaFields(arr, fieldName)
	if fields != nil && len(fields) >= minFields {
		return fields
	}
	return nil
}

func sharedArraySchema(arr []any, fieldName string) []string {
	// The first row must have this field so the decoder sees {fields} on row 0.
	if len(arr) > 0 {
		v, exists := objectItemGet(arr[0], fieldName)
		if !exists || v == nil {
			return nil
		}
		if _, ok := v.([]any); !ok {
			return nil
		}
	}
	var canonicalFields []string
	for _, item := range arr {
		v, exists := objectItemGet(item, fieldName)
		if !exists || v == nil {
			continue
		}
		arrVal, ok := v.([]any)
		if !ok {
			return nil
		}
		fields := tabularFields(arrVal)
		if fields == nil {
			return nil
		}
		// All values in the array items must be scalars for shared schema to work.
		for _, arrItem := range arrVal {
			keys := objectItemKeys(arrItem)
			if keys == nil {
				return nil
			}
			for _, k := range keys {
				val, _ := objectItemGet(arrItem, k)
				switch val.(type) {
				case *OrderedMap, map[string]any, []any:
					return nil
				}
			}
		}
		if canonicalFields == nil {
			canonicalFields = fields
		} else {
			if len(fields) != len(canonicalFields) {
				return nil
			}
			for i, f := range fields {
				if f != canonicalFields[i] {
					return nil
				}
			}
		}
	}
	return canonicalFields
}

// flatLeaf describes one leaf column produced by flattening a nested object.
type flatLeaf struct {
	path string   // ">" separated path for the header column name (e.g. "customer>name")
	keys []string // key chain to traverse from the row object
}

// analyzeFlattenable checks whether a field across all rows contains a fixed-shape
// nested object that can be flattened into path columns. Returns leaf descriptors
// if flattenable, nil otherwise. Recurses into nested objects.
func analyzeFlattenable(arr []any, fieldName string, parentPath string) []flatLeaf {
	// A field name containing ">" or empty cannot be flattened: it would create an
	// ambiguous path column (empty segment) the decoder treats as a literal field (SPEC 7.4.6.1.3).
	if fieldName == "" || strings.Contains(fieldName, ">") {
		return nil
	}
	type shapeEntry struct {
		kind string // "scalar" or "nested"
	}
	var canonicalKeys []string
	canonicalShape := make(map[string]shapeEntry)

	for _, item := range arr {
		v, exists := objectItemGet(item, fieldName)
		if !exists {
			continue
		}
		// A nested (non-top-level) null cannot be flattened losslessly: its leaves
		// would encode as absent ("~") and unflatten back to a missing key, not null.
		// Bail to the attachment path. A top-level null is fine (it emits "-" and
		// reconstructs via the all-null rule), so just skip the row from shape analysis.
		if v == nil {
			if parentPath != "" {
				return nil
			}
			continue
		}
		keys := objectItemKeys(v)
		if keys == nil {
			return nil // not an object (could be array or scalar)
		}
		// Check: arrays disqualify the field entirely.
		if _, ok := v.([]any); ok {
			return nil
		}

		if canonicalKeys == nil {
			canonicalKeys = keys
			for _, k := range keys {
				// A nested key containing ">" or empty cannot be flattened: empty
				// produces an empty path segment the decoder rejects as a path column (SPEC 7.4.6.1.3).
				if k == "" || strings.Contains(k, ">") {
					return nil
				}
				val, _ := objectItemGet(v, k)
				if val == nil {
					canonicalShape[k] = shapeEntry{kind: "scalar"}
					continue
				}
				switch val.(type) {
				case []any:
					return nil
				case *OrderedMap, map[string]any:
					canonicalShape[k] = shapeEntry{kind: "nested"}
				default:
					canonicalShape[k] = shapeEntry{kind: "scalar"}
				}
			}
		} else {
			if len(keys) != len(canonicalKeys) {
				return nil
			}
			for i, k := range keys {
				if k != canonicalKeys[i] {
					return nil
				}
				val, _ := objectItemGet(v, k)
				if val == nil {
					continue
				}
				expected := canonicalShape[k]
				if expected.kind == "scalar" {
					switch val.(type) {
					case *OrderedMap, map[string]any, []any:
						return nil
					}
				} else if expected.kind == "nested" {
					switch val.(type) {
					case []any:
						return nil
					case *OrderedMap, map[string]any:
						// ok
					default:
						// Was nested in first row but scalar here: inconsistent shape.
						return nil
					}
				}
			}
		}
	}

	if canonicalKeys == nil {
		return nil
	}

	currentPath := fieldName
	if parentPath != "" {
		currentPath = parentPath + ">" + fieldName
	}

	var leaves []flatLeaf
	parentKeys := []string{fieldName}
	if parentPath != "" {
		parentKeys = append(strings.Split(parentPath, ">"), fieldName)
	}

	for _, k := range canonicalKeys {
		entry := canonicalShape[k]
		if entry.kind == "scalar" {
			leaves = append(leaves, flatLeaf{
				path: currentPath + ">" + k,
				keys: append(append([]string{}, parentKeys...), k),
			})
		} else {
			// Nested: extract sub-objects from each row and recurse.
			subArr := make([]any, len(arr))
			for i, item := range arr {
				v, exists := objectItemGet(item, fieldName)
				if !exists || v == nil {
					subArr[i] = map[string]any{}
				} else {
					subArr[i] = v
				}
			}
			subLeaves := analyzeFlattenable(subArr, k, currentPath)
			if subLeaves == nil || len(subLeaves) == 0 {
				return nil // Empty nested object cannot be represented by flattening.
			}
			leaves = append(leaves, subLeaves...)
		}
	}

	// Guard against round-trip ambiguity: if any row has the field as a non-null
	// object where every scalar leaf is null, this is indistinguishable from the
	// parent-is-null encoding (all leaves emit "-"). Reject flattening in that case.
	if len(leaves) > 0 {
		for _, item := range arr {
			v, exists := objectItemGet(item, fieldName)
			if !exists || v == nil {
				continue
			}
			// v is a non-null object. Check if all leaves resolve to null.
			allNull := true
			for _, leaf := range leaves {
				// Build full key chain relative to item (prepend fieldName).
				val, leafExists := resolveKeyChain(item, leaf.keys)
				if !leafExists {
					// Absent leaf is not null.
					allNull = false
					break
				}
				if val != nil {
					allNull = false
					break
				}
			}
			if allNull {
				return nil
			}
		}
	}

	return leaves
}

// resolveKeyChain traverses an object following a key chain, returning the value
// and whether the top-level field exists. If the top-level field is absent, exists
// is false. If the top-level field is null, exists is true and val is nil.
func resolveKeyChain(item any, keys []string) (val any, exists bool) {
	if len(keys) == 0 {
		return nil, false
	}
	// Check top-level existence.
	current, topExists := objectItemGet(item, keys[0])
	if !topExists {
		return nil, false
	}
	if current == nil {
		return nil, true
	}
	for _, k := range keys[1:] {
		next, ok := objectItemGet(current, k)
		if !ok {
			return nil, false // leaf absent within nested object
		}
		current = next
	}
	return current, true
}

// flatColumn describes a column in the expanded (flattened) field list.
type flatColumn struct {
	headerName string   // formatted name for the header
	colType    string   // "scalar", "flat", or "attachment"
	field      string   // original field name
	keys       []string // key chain for flat columns
}

func encodeTabular(b *strings.Builder, headerPrefix string, arr []any, fields []string, depth int, opts encodeOpts, keyed bool) {
	prefix := indentStr(depth)

	// Phase 0: Analyze fields for flattening potential.
	flattenMap := make(map[string][]flatLeaf) // field -> leaves
	if opts.FlattenNested {
		for _, f := range fields {
			if leaves := analyzeFlattenable(arr, f, ""); leaves != nil && len(leaves) > 0 {
				flattenMap[f] = leaves
			}
		}
	}

	// Fields whose names contain ">" must not appear as tabular columns
	// because the decoder would interpret them as flattened path columns.
	// Track them for per-row attachment emission (spec rule 7.4.6.1.4).
	gtFields := make(map[string]bool)
	for _, f := range fields {
		if _, flattened := flattenMap[f]; !flattened && strings.Contains(f, ">") {
			gtFields[f] = true
		}
	}

	// Build expanded column list.
	var columns []flatColumn
	for _, f := range fields {
		if gtFields[f] {
			continue
		}
		if leaves, ok := flattenMap[f]; ok {
			for _, leaf := range leaves {
				columns = append(columns, flatColumn{
					headerName: formatKey(leaf.path),
					colType:    "flat",
					field:      f,
					keys:       leaf.keys,
				})
			}
		} else {
			columns = append(columns, flatColumn{
				headerName: formatKey(f),
				colType:    "", // determined per-row below
				field:      f,
			})
		}
	}

	// If all fields were excluded (all contain ">"), fall back to expanded.
	if len(columns) == 0 {
		encodeExpanded(b, headerPrefix, arr, depth, opts)
		return
	}

	// Compute inline schemas and shared array schemas for non-flattened fields only.
	inlineSchemas := make(map[string][]string)
	if opts.InlineObjectSchema {
		minF := opts.MinInlineFields
		if minF <= 0 {
			minF = 1
		}
		for _, f := range fields {
			if _, flattened := flattenMap[f]; flattened {
				continue
			}
			if ifs := inlineSchemaFieldsMin(arr, f, minF); ifs != nil {
				inlineSchemas[f] = ifs
			}
		}
	}

	sharedArrSchemas := make(map[string][]string)
	if opts.SharedArraySchema {
		for _, f := range fields {
			if _, flattened := flattenMap[f]; flattened {
				continue
			}
			if sas := sharedArraySchema(arr, f); sas != nil {
				sharedArrSchemas[f] = sas
			}
		}
	}

	// Format header.
	headerFields := make([]string, len(columns))
	for i, col := range columns {
		headerFields[i] = col.headerName
	}
	br := "]"
	if keyed {
		br = ":]"
	}
	fmt.Fprintf(b, "%s[%d%s{%s}\n", headerPrefix, len(arr), br, strings.Join(headerFields, ","))

	// Encode rows.
	for i, item := range arr {
		cells := make([]string, len(columns))
		var attachments []fieldAttachment
		rowHasAttachment := false

		for j, col := range columns {
			if col.colType == "flat" {
				// Resolve value via key chain.
				val, exists := resolveKeyChain(item, col.keys)
				if !exists {
					cells[j] = "~"
				} else if val == nil {
					cells[j] = "-"
				} else {
					cells[j] = formatScalar(val, '|')
				}
				continue
			}

			// Non-flattened field: original logic.
			f := col.field
			v, exists := objectItemGet(item, f)
			if !exists {
				cells[j] = "~"
				continue
			}
			if v == nil {
				cells[j] = "-"
				continue
			}
			switch nested := v.(type) {
			case *OrderedMap:
				if ifs, ok := inlineSchemas[f]; ok {
					if i == 0 {
						fmtIF := make([]string, len(ifs))
						for k, inf := range ifs {
							fmtIF[k] = formatKey(inf)
						}
						cells[j] = "^{" + strings.Join(fmtIF, ",") + "}"
					} else {
						cells[j] = "^"
					}
					attachments = append(attachments, fieldAttachment{name: f, value: nested, inline: true, inlineFields: ifs})
				} else {
					cells[j] = "^"
					attachments = append(attachments, fieldAttachment{name: f, value: nested})
				}
				rowHasAttachment = true
			case map[string]any:
				if ifs, ok := inlineSchemas[f]; ok {
					if i == 0 {
						fmtIF := make([]string, len(ifs))
						for k, inf := range ifs {
							fmtIF[k] = formatKey(inf)
						}
						cells[j] = "^{" + strings.Join(fmtIF, ",") + "}"
					} else {
						cells[j] = "^"
					}
					attachments = append(attachments, fieldAttachment{name: f, value: nested, inline: true, inlineFields: ifs})
				} else {
					cells[j] = "^"
					attachments = append(attachments, fieldAttachment{name: f, value: nested})
				}
				rowHasAttachment = true
			case []any:
				cells[j] = "^"
				attachments = append(attachments, fieldAttachment{name: f, value: nested})
				rowHasAttachment = true
			default:
				cells[j] = formatScalar(v, '|')
			}
		}

		// Emit fields with ">" in their names as per-row attachments.
		for _, f := range fields {
			if !gtFields[f] {
				continue
			}
			v, exists := objectItemGet(item, f)
			if !exists {
				continue
			}
			rowHasAttachment = true
			attachments = append(attachments, fieldAttachment{name: f, value: v})
		}

		row := strings.Join(cells, "|")
		if rowHasAttachment {
			fmt.Fprintf(b, "%s@%d %s\n", prefix, i, row)
		} else {
			b.WriteString(prefix)
			b.WriteString(row)
			b.WriteByte('\n')
		}

		for _, att := range attachments {
			attIndent := prefix + "  "
			if opts.DropAttachIndent {
				attIndent = prefix
			}
			fk := formatKey(att.name)

			if att.inline {
				vals := make([]string, len(att.inlineFields))
				for k, inf := range att.inlineFields {
					v, exists := objectItemGet(att.value, inf)
					if !exists {
						vals[k] = "~"
					} else {
						vals[k] = formatScalar(v, '|')
					}
				}
				if opts.DropFieldPrefix {
					fmt.Fprintf(b, "%s%s\n", attIndent, strings.Join(vals, "|"))
				} else {
					fmt.Fprintf(b, "%s.%s %s\n", attIndent, fk, strings.Join(vals, "|"))
				}
			} else {
				switch av := att.value.(type) {
				case *OrderedMap:
					if ks, vs, vf, kl, ok := keyedMapEligible(av, opts); ok {
						encodeKeyedMapWithPrefix(b, fmt.Sprintf("%s.%s ", attIndent, fk), ks, vs, vf, kl, depth+2, opts)
						break
					}
					fmt.Fprintf(b, "%s.%s {}\n", attIndent, fk)
					encodeOrderedObject(b, av, depth+2, opts)
				case map[string]any:
					if ks, vs, vf, kl, ok := keyedMapEligible(av, opts); ok {
						encodeKeyedMapWithPrefix(b, fmt.Sprintf("%s.%s ", attIndent, fk), ks, vs, vf, kl, depth+2, opts)
						break
					}
					fmt.Fprintf(b, "%s.%s {}\n", attIndent, fk)
					encodeObject(b, av, depth+2, opts)
				case []any:
					if sas, ok := sharedArrSchemas[att.name]; ok && i > 0 {
						encodeAttachmentArrayShared(b, attIndent, fk, av, depth+2, opts, sas)
					} else {
						encodeAttachmentArray(b, attIndent, fk, av, depth+2, opts)
					}
				default:
					// Scalar attachment (e.g. field names containing ">").
					if att.value == nil {
						fmt.Fprintf(b, "%s.%s =-\n", attIndent, fk)
					} else {
						fmt.Fprintf(b, "%s.%s =%s\n", attIndent, fk, formatScalar(att.value, '\x00'))
					}
				}
			}
		}
	}
}

func encodeAttachmentArray(b *strings.Builder, attPrefix, fk string, arr []any, depth int, opts encodeOpts) {
	if len(arr) == 0 {
		fmt.Fprintf(b, "%s.%s [0]\n", attPrefix, fk)
	} else if allPrimitives(arr) {
		parts := make([]string, len(arr))
		for i, v := range arr {
			parts[i] = formatScalar(v, ',')
		}
		fmt.Fprintf(b, "%s.%s [%d]: %s\n", attPrefix, fk, len(arr), strings.Join(parts, ","))
	} else if nestedFields := tabularFields(arr); nestedFields != nil {
		encodeTabular(b, fmt.Sprintf("%s.%s ", attPrefix, fk), arr, nestedFields, depth, opts, false)
	} else {
		encodeExpanded(b, fmt.Sprintf("%s.%s ", attPrefix, fk), arr, depth, opts)
	}
}

func encodeAttachmentArrayShared(b *strings.Builder, attPrefix, fk string, arr []any, depth int, opts encodeOpts, sharedFields []string) {
	if len(arr) == 0 {
		fmt.Fprintf(b, "%s.%s [0]\n", attPrefix, fk)
	} else if allPrimitives(arr) {
		parts := make([]string, len(arr))
		for i, v := range arr {
			parts[i] = formatScalar(v, ',')
		}
		fmt.Fprintf(b, "%s.%s [%d]: %s\n", attPrefix, fk, len(arr), strings.Join(parts, ","))
	} else if nf := tabularFields(arr); nf != nil && fieldsMatch(nf, sharedFields) {
		prefix := indentStr(depth)
		fmt.Fprintf(b, "%s.%s [%d]\n", attPrefix, fk, len(arr))
		for _, item := range arr {
			cells := make([]string, len(sharedFields))
			for j, f := range sharedFields {
				v, exists := objectItemGet(item, f)
				if !exists {
					cells[j] = "~"
				} else if v == nil {
					cells[j] = "-"
				} else {
					cells[j] = formatScalar(v, '|')
				}
			}
			b.WriteString(prefix)
			b.WriteString(strings.Join(cells, "|"))
			b.WriteByte('\n')
		}
	} else {
		// Fields don't match shared schema: fall back to full encoder with {fields}.
		encodeAttachmentArray(b, attPrefix, fk, arr, depth, opts)
	}
}

func fieldsMatch(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
