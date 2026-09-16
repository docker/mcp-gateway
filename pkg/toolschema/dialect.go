// Package toolschema translates the JSON Schema documents that upstream MCP
// servers attach to their tools into the 2020-12 dialect.
//
// Neither the servers nor the clients involved are misbehaving, and that is why
// the fix belongs here. The MCP 2026-07-28 spec's "JSON Schema Usage" section
// lets a schema declare any dialect, requires implementations to support only
// 2020-12, and tells them to handle an unsupported dialect by returning an
// error saying so -- which is exactly what the rejecting clients do. Servers
// built on the MCP TypeScript SDK declare draft-07 for every tool, because its
// zod converter defaults to that target. Both halves are conformant and the
// combination does not work, so the gateway in the middle reconciles them.
//
// That section postdates the revision this gateway negotiates (the vendored
// go-sdk tops out at 2025-06-18, whose basic/index.mdx says nothing normative
// about dialects), so it is cited as the ecosystem's settled reading of a rule
// that was always implicit, not as a requirement binding the current wire
// version. The breakage it describes is happening today either way.
//
// Translation is deliberately narrow. Normalize only acts on a schema that
// explicitly declares draft-04, draft-06 or draft-07; a schema with no $schema
// is already 2020-12 by the spec's default and is returned untouched rather
// than guessed at. Where a construct has no faithful 2020-12 equivalent the
// whole schema is returned unchanged, so the caller's fallback is the verbatim
// passthrough that predates this package rather than a plausible-looking lie.
//
// Input must be a JSON-decoded document, as every schema on the MCP wire is:
// containers are map[string]any and []any, and the values inside them are JSON
// scalars. Normalize deep-copies the containers it returns, so a JSON-decoded
// input is never mutated and the result aliases none of it. A hand-built schema
// holding some other container type is copied by reference instead, which is
// harmless only because nothing in the gateway writes through a tool schema.
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
	// schema accepts. Anything else beside a $ref is an assertion, and a
	// pre-2020-12 dialect ignores it where 2020-12 applies it.
	//
	// $id is deliberately NOT here. Under 2020-12 an $id beside a $ref sets the
	// base URI the $ref resolves against, so it can change which schema the
	// reference names -- not an annotation, and not safe to wave through.
	refAnnotationSiblings = map[string]bool{
		"$comment":    true,
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
	// UnsupportedDialect is true when the schema declared a dialect that is
	// neither 2020-12 nor one this package translates, so it was returned
	// unchanged. Such a schema is still rejected by a 2020-12-only client, and a
	// caller sizing the affected surface has to be able to see it -- otherwise
	// "nothing to report" and "affected but beyond our reach" look identical.
	UnsupportedDialect bool
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
		return schema, Result{UnsupportedDialect: declared != Dialect202012 && declared != Dialect202012+"#"}
	}

	if reason := withinBudget(root); reason != "" {
		return schema, Result{Skipped: true, Reason: reason}
	}
	if reason := untranslatableSchema(root, from, true); reason != "" {
		return schema, Result{Skipped: true, Reason: reason}
	}

	converted := convert(root, from).(map[string]any)
	converted["$schema"] = Dialect202012
	return converted, Result{Changed: true}
}

// Budget on how much of an upstream document this package will walk.
//
// A tool schema is attacker-controlled input from a trust boundary's far side,
// and MCP 2026-07-28 asks implementations to bound schema depth or node count
// for exactly this reason (basic/index.mdx, "Composition-Keyword Resource
// Use"). Before this package existed the gateway relayed schemas opaquely and
// never walked them, so the traversal is new exposure and carries its own
// limit. Both limits are orders of magnitude above the hand-written and
// zod-generated tool schemas this package was built for, which nest a handful
// of levels; they are a ceiling on a hostile document, not a design constraint.
const (
	maxSchemaDepth = 64
	maxSchemaNodes = 20000
)

// withinBudget reports a reason to leave the schema alone when it is too deep
// or too large to walk. It counts every JSON value rather than only subschema
// positions, so the cost of the check cannot be inflated by burying the
// structure under keywords the translator ignores.
func withinBudget(node any) string {
	nodes := 0
	var walk func(value any, depth int) string
	walk = func(value any, depth int) string {
		if depth > maxSchemaDepth {
			return "schema nests deeper than the translator will walk"
		}
		nodes++
		if nodes > maxSchemaNodes {
			return "schema holds more values than the translator will walk"
		}
		switch v := value.(type) {
		case map[string]any:
			for _, member := range v {
				if reason := walk(member, depth+1); reason != "" {
					return reason
				}
			}
		case []any:
			for _, item := range v {
				if reason := walk(item, depth+1); reason != "" {
					return reason
				}
			}
		}
		return ""
	}
	return walk(node, 0)
}

// untranslatableSchema walks a schema node and reports the first construct
// whose meaning a relabel to 2020-12 would change, or that the converter cannot
// express. Every case is ambiguity rather than difficulty, and the answer to
// all of them is to leave the schema exactly as the server sent it.
//
// The traversal mirrors convert's, descending through the typed helpers rather
// than walking every map it meets. A name-to-subschema map must not be treated
// as a schema node, or a property named "$ref" would read as a $ref whose
// siblings are the other property names.
func untranslatableSchema(node any, from draft, isRoot bool) string {
	n, ok := node.(map[string]any)
	if !ok {
		return ""
	}

	if !isRoot {
		// An embedded resource declaring its own dialect is its own translation
		// problem, governed by its own $schema. Converting its body under the
		// root's rules while leaving its declaration in place would produce a
		// document whose two halves disagree.
		if _, nested := n["$schema"]; nested {
			return "an embedded subschema declares its own $schema"
		}
	}

	if _, hasRef := n["$ref"]; hasRef {
		for key := range n {
			if key == "$ref" || refAnnotationSiblings[key] {
				continue
			}
			return "$ref carries the assertion keyword " + key +
				", which pre-2020-12 dialects ignore and 2020-12 enforces"
		}
	}

	// A keyword introduced after the declared dialect is inert where the schema
	// was written and live once it claims 2020-12, so relabelling would start
	// enforcing something the author's own validator never applied. This also
	// covers a schema mixing a pre-2020-12 keyword with the 2020-12 keyword the
	// converter would rewrite it into (dependencies beside dependentRequired, a
	// tuple items beside prefixItems), which would otherwise collide.
	for key := range n {
		if post202012Keywords[key] {
			return "schema declares " + key + ", which the declared dialect ignores and 2020-12 enforces"
		}
	}

	// A plain-name fragment is a legal $id in draft-06 and draft-07 and is
	// forbidden in 2020-12, which spells the same thing $anchor. Rewriting it
	// would have to repoint every $ref that names it.
	if id, ok := n[idKeyword(from)].(string); ok && isPlainNameFragment(id) {
		return "schema uses a plain-name fragment in " + idKeyword(from) + ", which 2020-12 spells with $anchor"
	}

	if from == draft4 {
		if _, hasDollarID := n["$id"]; hasDollarID {
			if _, hasID := n["id"]; hasID {
				// Renaming id to $id would collide, and which one survived
				// would depend on map iteration order.
				return "draft-04 schema declares both id and $id"
			}
		}
		for _, pair := range draft4ExclusiveBounds {
			exclusive, isBool := n[pair.flag].(bool)
			if !isBool || !exclusive {
				continue
			}
			if _, hasBound := n[pair.bound]; !hasBound {
				// draft-04 requires the bound the flag modifies. Without it
				// there is no value to carry across, and dropping the flag
				// would silently widen what the schema accepts.
				return "draft-04 schema sets " + pair.flag + " with no " + pair.bound
			}
		}
	}

	if deps, hasDependencies := n["dependencies"]; hasDependencies {
		if _, isMap := deps.(map[string]any); !isMap {
			// Not the shape dependencies is defined to have, so it cannot be
			// split into the two keywords that replaced it.
			return "schema declares a dependencies value that is not an object"
		}
	}

	for key, value := range n {
		switch {
		case key == "items":
			// Either a tuple (a list of subschemas) or a single subschema.
			if tuple, isTuple := value.([]any); isTuple {
				if reason := untranslatableList(tuple, from); reason != "" {
					return reason
				}
				continue
			}
			if reason := untranslatableSchema(value, from, false); reason != "" {
				return reason
			}
		case key == "dependencies":
			// A name-to-(subschema | required list) map.
			if reason := untranslatableMap(value, from); reason != "" {
				return reason
			}
		case schemaMapKeywords[key]:
			if reason := untranslatableMap(value, from); reason != "" {
				return reason
			}
		case schemaListKeywords[key]:
			if reason := untranslatableList(value, from); reason != "" {
				return reason
			}
		case singleSchemaKeywords[key]:
			// convert dispatches on the value's type here, so the guard has to
			// as well: a single-schema keyword holding a list still gets
			// rewritten, and a guard that stopped at the non-map would let a
			// bail-out condition inside it through.
			if reason := untranslatableValue(value, from); reason != "" {
				return reason
			}
		}
	}
	return ""
}

// untranslatableValue guards a value in a subschema position, dispatching on
// its type exactly as convert does.
func untranslatableValue(value any, from draft) string {
	switch value.(type) {
	case []any:
		return untranslatableList(value, from)
	default:
		return untranslatableSchema(value, from, false)
	}
}

func untranslatableMap(value any, from draft) string {
	members, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	for _, member := range members {
		if reason := untranslatableSchema(member, from, false); reason != "" {
			return reason
		}
	}
	return ""
}

func untranslatableList(value any, from draft) string {
	items, ok := value.([]any)
	if !ok {
		return ""
	}
	for _, item := range items {
		if reason := untranslatableSchema(item, from, false); reason != "" {
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
	for _, pair := range draft4ExclusiveBounds {
		exclusive, isBool := in[pair.flag].(bool)
		if !isBool || !exclusive {
			continue
		}
		bound, has := in[pair.bound]
		if !has {
			// untranslatableSchema refuses this shape, so reaching here means
			// the guard and the converter have drifted apart.
			continue
		}
		out[pair.flag] = deepCopy(bound)
		delete(out, pair.bound)
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

// deepCopy copies the containers a JSON-decoded document is made of, so the
// returned schema shares no mutable state with the input. JSON scalars are
// immutable and are shared. A container of some other type -- map[string]string,
// say -- is returned by reference; see the package comment for why that cannot
// occur on the path this package is used on.
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

// post202012Keywords are keywords that no dialect this package translates
// defines, and that 2020-12 (or 2019-09, which 2020-12 inherits) does. In a
// schema declaring an older dialect they are inert annotations the author's own
// validator ignored; relabelling the document to 2020-12 would make them live.
//
// prefixItems, dependentRequired and dependentSchemas are here for that reason
// and for a second one: they are the keywords the converter rewrites tuple
// items and dependencies into, so a schema already carrying them has nowhere
// for the rewrite to land.
var post202012Keywords = map[string]bool{
	"$anchor":               true,
	"$dynamicAnchor":        true,
	"$dynamicRef":           true,
	"$recursiveAnchor":      true,
	"$recursiveRef":         true,
	"$vocabulary":           true,
	"dependentRequired":     true,
	"dependentSchemas":      true,
	"maxContains":           true,
	"minContains":           true,
	"prefixItems":           true,
	"unevaluatedItems":      true,
	"unevaluatedProperties": true,
}

// draft4ExclusiveBounds pairs draft-04's boolean exclusivity flags with the
// inclusive bound each one modifies. A slice rather than a map so it is not
// rebuilt at every node of every draft-04 schema, and so the order is fixed.
var draft4ExclusiveBounds = []struct{ flag, bound string }{
	{flag: "exclusiveMinimum", bound: "minimum"},
	{flag: "exclusiveMaximum", bound: "maximum"},
}

// idKeyword names the identifier keyword of a dialect. draft-04 spells it "id";
// draft-06 renamed it to "$id".
func idKeyword(from draft) string {
	if from == draft4 {
		return "id"
	}
	return "$id"
}

// isPlainNameFragment reports whether an identifier is a bare fragment such as
// "#row", which draft-06 and draft-07 allow as an $id and 2020-12 does not.
// A fragment that is empty or a JSON Pointer is not a plain name.
func isPlainNameFragment(id string) bool {
	rest, ok := strings.CutPrefix(id, "#")
	return ok && rest != "" && rest[0] != '/'
}
