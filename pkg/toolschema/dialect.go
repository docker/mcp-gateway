// Package toolschema translates the JSON Schema documents that upstream MCP
// servers attach to their tools into the 2020-12 dialect.
//
// MCP permits a tool schema to declare any dialect and only requires
// implementations to support 2020-12 (2026-07-28 spec, "JSON Schema Usage").
// Servers built on the TypeScript SDK emit draft-07, because the zod converter
// defaults to that target, so a client whose validator is configured for
// 2020-12 alone rejects every one of their tools before dispatch.
//
// Translation is deliberately narrow. Normalize only acts on a schema that
// explicitly declares draft-04, draft-06 or draft-07; a schema with no $schema
// is already 2020-12 by the spec's default and is returned untouched rather
// than guessed at. Where a construct has no faithful 2020-12 equivalent the
// whole schema is returned unchanged, so the caller's fallback is the verbatim
// passthrough that predates this package rather than a plausible-looking lie.
package toolschema

import "strings"

// Dialect202012 is the 2020-12 dialect URI, which MCP treats as the default.
const Dialect202012 = "https://json-schema.org/draft/2020-12/schema"

// translatableDialects are the pre-2020-12 dialects Normalize can convert. Both
// the http and https spellings occur in the wild, with and without the trailing
// empty fragment.
var translatableDialects = map[string]draft{
	"http://json-schema.org/draft-07/schema#":  draft7,
	"http://json-schema.org/draft-07/schema":   draft7,
	"https://json-schema.org/draft-07/schema#": draft7,
	"https://json-schema.org/draft-07/schema":  draft7,
	"http://json-schema.org/draft-06/schema#":  draft6,
	"http://json-schema.org/draft-06/schema":   draft6,
	"https://json-schema.org/draft-06/schema#": draft6,
	"https://json-schema.org/draft-06/schema":  draft6,
	"http://json-schema.org/draft-04/schema#":  draft4,
	"http://json-schema.org/draft-04/schema":   draft4,
	"https://json-schema.org/draft-04/schema#": draft4,
	"https://json-schema.org/draft-04/schema":  draft4,
}

type draft int

const (
	draft4 draft = iota
	draft6
	draft7
)

// Keyword classification. Recursion descends only into these; every other
// keyword's value is copied as opaque data, which keeps enum, const, default
// and examples payloads that happen to contain schema-shaped objects from being
// rewritten.
var (
	// singleSchemaKeywords hold exactly one subschema.
	singleSchemaKeywords = map[string]bool{
		"additionalItems":       true,
		"additionalProperties":  true,
		"contains":              true,
		"contentSchema":         true,
		"else":                  true,
		"if":                    true,
		"items":                 true,
		"not":                   true,
		"propertyNames":         true,
		"then":                  true,
		"unevaluatedItems":      true,
		"unevaluatedProperties": true,
	}

	// schemaListKeywords hold an array of subschemas.
	schemaListKeywords = map[string]bool{
		"allOf":       true,
		"anyOf":       true,
		"oneOf":       true,
		"prefixItems": true,
	}

	// schemaMapKeywords hold a name-to-subschema map.
	schemaMapKeywords = map[string]bool{
		"$defs":             true,
		"definitions":       true,
		"dependentSchemas":  true,
		"patternProperties": true,
		"properties":        true,
	}

	// refAnnotationSiblings may appear beside $ref without changing what the
	// schema accepts. Anything else beside a $ref is an assertion, and draft-07
	// ignores it where 2020-12 applies it.
	refAnnotationSiblings = map[string]bool{
		"$comment":    true,
		"$id":         true,
		"default":     true,
		"deprecated":  true,
		"description": true,
		"examples":    true,
		"readOnly":    true,
		"title":       true,
		"writeOnly":   true,
	}
)

// Result reports what Normalize did to a schema.
type Result struct {
	// Changed is true when the returned schema differs from the input.
	Changed bool
	// Skipped is true when the schema declared a translatable dialect but held
	// a construct with no faithful 2020-12 equivalent, so it was returned
	// unchanged. Reason says which.
	Skipped bool
	// Reason explains a Skipped result. Empty otherwise.
	Reason string
}

// Normalize returns a 2020-12 equivalent of schema.
//
// It never mutates schema or anything reachable from it: MCP tool schemas are
// shared by reference with the upstream client session's cached tool list, and
// discovery runs concurrently across servers.
//
// A schema that declares no dialect, declares 2020-12, or declares a dialect
// this package does not translate is returned as-is.
func Normalize(schema any) (any, Result) {
	root, ok := schema.(map[string]any)
	if !ok {
		// Boolean schemas, nil, and typed *jsonschema.Schema values carry no
		// dialect declaration to act on.
		return schema, Result{}
	}

	declared, ok := root["$schema"].(string)
	if !ok {
		return schema, Result{}
	}
	from, ok := translatableDialects[declared]
	if !ok {
		return schema, Result{}
	}

	if reason := untranslatableSchema(root); reason != "" {
		return schema, Result{Skipped: true, Reason: reason}
	}

	converted := convert(root, from).(map[string]any)
	converted["$schema"] = Dialect202012
	return converted, Result{Changed: true}
}

// untranslatableSchema walks a schema node and reports the first construct that
// cannot be converted without changing what the schema accepts. The four cases
// are all ambiguity rather than difficulty: a $ref whose siblings one dialect
// ignores and the other enforces, and three shapes that mix a draft-07 keyword
// with the 2020-12 keyword it would be rewritten into.
//
// The traversal mirrors convert's, descending through the typed helpers rather
// than walking every map it meets. A name-to-subschema map must not be treated
// as a schema node, or a property named "$ref" would read as a $ref whose
// siblings are the other property names.
func untranslatableSchema(node any) string {
	n, ok := node.(map[string]any)
	if !ok {
		return ""
	}

	if _, hasRef := n["$ref"]; hasRef {
		for key := range n {
			if key == "$ref" || refAnnotationSiblings[key] {
				continue
			}
			return "$ref carries the assertion keyword " + key + ", which draft-07 ignores and 2020-12 enforces"
		}
	}
	if _, isTuple := n["items"].([]any); isTuple {
		if _, hasPrefix := n["prefixItems"]; hasPrefix {
			return "schema declares both a draft-07 tuple items and prefixItems"
		}
	}
	if _, hasDefinitions := n["definitions"]; hasDefinitions {
		if _, hasDefs := n["$defs"]; hasDefs {
			return "schema declares both definitions and $defs"
		}
	}
	if _, hasDependencies := n["dependencies"]; hasDependencies {
		if _, has := n["dependentRequired"]; has {
			return "schema declares both dependencies and dependentRequired"
		}
		if _, has := n["dependentSchemas"]; has {
			return "schema declares both dependencies and dependentSchemas"
		}
	}

	for key, value := range n {
		switch {
		case key == "items":
			// Either a tuple (a list of subschemas) or a single subschema.
			if tuple, isTuple := value.([]any); isTuple {
				if reason := untranslatableList(tuple); reason != "" {
					return reason
				}
				continue
			}
			if reason := untranslatableSchema(value); reason != "" {
				return reason
			}
		case key == "dependencies":
			// A name-to-(subschema | required list) map.
			if reason := untranslatableMap(value); reason != "" {
				return reason
			}
		case schemaMapKeywords[key]:
			if reason := untranslatableMap(value); reason != "" {
				return reason
			}
		case schemaListKeywords[key]:
			if reason := untranslatableList(value); reason != "" {
				return reason
			}
		case singleSchemaKeywords[key]:
			if reason := untranslatableSchema(value); reason != "" {
				return reason
			}
		}
	}
	return ""
}

func untranslatableMap(value any) string {
	members, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	for _, member := range members {
		if reason := untranslatableSchema(member); reason != "" {
			return reason
		}
	}
	return ""
}

func untranslatableList(value any) string {
	items, ok := value.([]any)
	if !ok {
		return ""
	}
	for _, item := range items {
		if reason := untranslatableSchema(item); reason != "" {
			return reason
		}
	}
	return ""
}

// convert returns a translated deep copy of node.
func convert(node any, from draft) any {
	switch n := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(n)+1)
		for key, value := range n {
			switch {
			case key == "$ref":
				out[key] = rewriteRef(value)
			case key == "items":
				// A draft-07 tuple becomes prefixItems, and additionalItems
				// becomes the 2020-12 items that governs the rest of the array.
				if tuple, isTuple := value.([]any); isTuple {
					out["prefixItems"] = convertList(tuple, from)
					if rest, has := n["additionalItems"]; has {
						out["items"] = convert(rest, from)
					}
					continue
				}
				out[key] = convert(value, from)
			case key == "additionalItems":
				// Only meaningful beside a tuple items, handled above. Beside a
				// single-schema or absent items, draft-07 ignores it, so
				// dropping it preserves what the schema accepts.
				continue
			case key == "definitions":
				out["$defs"] = convertMap(value, from)
			case key == "dependencies":
				required, schemas := splitDependencies(value, from)
				if len(required) > 0 {
					out["dependentRequired"] = required
				}
				if len(schemas) > 0 {
					out["dependentSchemas"] = schemas
				}
			case key == "id" && from == draft4:
				out["$id"] = deepCopy(value)
			case (key == "exclusiveMinimum" || key == "exclusiveMaximum") && from == draft4:
				// draft-04 spells these as booleans modifying minimum/maximum;
				// 2020-12 spells them as the bound itself. Only the boolean
				// form is rewritten, by convertDraft4Bounds below. A numeric
				// value in a draft-04-declaring schema is already the 2020-12
				// form, so it is carried across rather than dropped.
				if _, isBool := value.(bool); isBool {
					continue
				}
				out[key] = deepCopy(value)
			case schemaMapKeywords[key]:
				out[key] = convertMap(value, from)
			case schemaListKeywords[key]:
				out[key] = convertList(value, from)
			case singleSchemaKeywords[key]:
				out[key] = convert(value, from)
			default:
				out[key] = deepCopy(value)
			}
		}
		if from == draft4 {
			convertDraft4Bounds(n, out)
		}
		return out
	case []any:
		return convertList(n, from)
	default:
		return node
	}
}

// convertDraft4Bounds folds draft-04's boolean exclusiveMinimum/exclusiveMaximum
// into the 2020-12 numeric form. A true flag moves the bound across; a false or
// absent flag leaves the inclusive bound alone.
func convertDraft4Bounds(in map[string]any, out map[string]any) {
	for flagKey, boundKey := range map[string]string{
		"exclusiveMinimum": "minimum",
		"exclusiveMaximum": "maximum",
	} {
		exclusive, isBool := in[flagKey].(bool)
		if !isBool || !exclusive {
			continue
		}
		bound, has := in[boundKey]
		if !has {
			continue
		}
		out[flagKey] = deepCopy(bound)
		delete(out, boundKey)
	}
}

// splitDependencies divides draft-07's dependencies into the two 2020-12
// keywords it was split into: an array value is a required-property list, any
// other value is a subschema.
func splitDependencies(value any, from draft) (map[string]any, map[string]any) {
	deps, ok := value.(map[string]any)
	if !ok {
		return nil, nil
	}
	required := map[string]any{}
	schemas := map[string]any{}
	for name, dep := range deps {
		if list, isList := dep.([]any); isList {
			required[name] = deepCopy(list)
			continue
		}
		schemas[name] = convert(dep, from)
	}
	return required, schemas
}

// rewriteRef repoints a same-document pointer at the renamed $defs section.
// untranslatable has already established that no node mixes definitions with
// $defs, so every /definitions/ segment in a local pointer was renamed.
func rewriteRef(value any) any {
	ref, ok := value.(string)
	if !ok || !strings.HasPrefix(ref, "#/") {
		return deepCopy(value)
	}
	return strings.ReplaceAll(ref, "/definitions/", "/$defs/")
}

func convertMap(value any, from draft) any {
	members, ok := value.(map[string]any)
	if !ok {
		return deepCopy(value)
	}
	out := make(map[string]any, len(members))
	for name, member := range members {
		out[name] = convert(member, from)
	}
	return out
}

func convertList(value any, from draft) any {
	items, ok := value.([]any)
	if !ok {
		return deepCopy(value)
	}
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = convert(item, from)
	}
	return out
}

// deepCopy copies containers so the returned schema shares no mutable state
// with the input. JSON scalars are immutable and are shared.
func deepCopy(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = deepCopy(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = deepCopy(item)
		}
		return out
	default:
		return value
	}
}
