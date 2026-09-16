package toolschema

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const draft7URI = "http://json-schema.org/draft-07/schema#"

// airtableListBasesOutputSchema is the outputSchema the Airtable MCP server
// advertises for list_bases, captured verbatim from a tools/list round trip
// against @modelcontextprotocol/sdk 1.24.3 with zod 4. It is the schema in the
// customer report that motivated this package.
const airtableListBasesOutputSchema = `{
  "type": "object",
  "properties": {
    "bases": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "name": {"type": "string"},
          "permissionLevel": {"type": "string"}
        },
        "required": ["id", "name", "permissionLevel"],
        "additionalProperties": false
      }
    }
  },
  "required": ["bases"],
  "$schema": "http://json-schema.org/draft-07/schema#",
  "additionalProperties": false
}`

func parse(t *testing.T, doc string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(doc), &out))
	return out
}

func normalized(t *testing.T, doc string) map[string]any {
	t.Helper()
	out, res := Normalize(parse(t, doc))
	require.True(t, res.Changed, "expected the schema to be translated")
	require.False(t, res.Skipped, "unexpected skip: %s", res.Reason)
	asMap, ok := out.(map[string]any)
	require.True(t, ok, "Normalize returned %T, want map[string]any", out)
	return asMap
}

func TestNormalizeRelabelsTheDialect(t *testing.T) {
	out := normalized(t, airtableListBasesOutputSchema)
	require.Equal(t, Dialect202012, out["$schema"])

	// Nothing but the dialect should move on a schema this plain: the body is
	// already valid 2020-12 and only the declaration was making clients reject
	// it. Compare with $schema removed from both sides.
	in := parse(t, airtableListBasesOutputSchema)
	delete(in, "$schema")
	got := map[string]any{}
	for k, v := range out {
		if k != "$schema" {
			got[k] = v
		}
	}
	require.Equal(t, in, got)
}

// TestNormalizeRewritesTupleItems is the case that a strip-only or relabel-only
// implementation cannot pass. zod emits draft-07 tuple form for z.tuple(...),
// and array-form items is not merely deprecated under 2020-12 but invalid:
// ajv's 2020-12 instance rejects the schema with
// "schema is invalid: data/items must be object,boolean". Relabelling alone
// therefore trades a dialect error for a schema-invalid error.
func TestNormalizeRewritesTupleItems(t *testing.T) {
	out := normalized(t, `{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "type": "array",
	  "items": [{"type": "string"}, {"type": "number"}],
	  "additionalItems": false,
	  "minItems": 2,
	  "maxItems": 2
	}`)

	// additionalItems becomes the 2020-12 items, and the plain constraints ride
	// along untouched.
	require.JSONEq(t, `{
	  "$schema": "`+Dialect202012+`",
	  "type": "array",
	  "prefixItems": [{"type": "string"}, {"type": "number"}],
	  "items": false,
	  "minItems": 2,
	  "maxItems": 2
	}`, mustJSON(t, out))
}

func TestNormalizeDropsAdditionalItemsBesideASingleItemsSchema(t *testing.T) {
	// draft-07 ignores additionalItems unless items is an array, so carrying it
	// into 2020-12 (where it is not a keyword at all) is pointless, and leaving
	// it beside the translated items would read as a constraint.
	out := normalized(t, `{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "type": "array",
	  "items": {"type": "string"},
	  "additionalItems": false
	}`)
	require.Equal(t, map[string]any{"type": "string"}, out["items"])
	require.NotContains(t, out, "additionalItems")
	require.NotContains(t, out, "prefixItems")
}

func TestNormalizeSplitsDependencies(t *testing.T) {
	// 2020-12 keeps no "dependencies" keyword. A 2020-12 validator treats it as
	// an unknown keyword and ignores it, so leaving it in place silently drops
	// the constraint rather than failing loudly.
	out := normalized(t, `{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "type": "object",
	  "dependencies": {
	    "creditCard": ["billingAddress"],
	    "shipping": {"required": ["address"]}
	  }
	}`)

	require.Equal(t, map[string]any{"creditCard": []any{"billingAddress"}}, out["dependentRequired"])
	require.Equal(t, map[string]any{"shipping": map[string]any{"required": []any{"address"}}}, out["dependentSchemas"])
	require.NotContains(t, out, "dependencies")
}

func TestNormalizeRenamesDefinitionsAndRepointsRefs(t *testing.T) {
	out := normalized(t, `{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "type": "object",
	  "definitions": {"row": {"type": "string"}},
	  "properties": {
	    "a": {"$ref": "#/definitions/row"},
	    "b": {"$ref": "https://example.test/schema#/definitions/row"}
	  }
	}`)

	require.Equal(t, map[string]any{"row": map[string]any{"type": "string"}}, out["$defs"])
	require.NotContains(t, out, "definitions")

	props := out["properties"].(map[string]any)
	require.Equal(t, "#/$defs/row", props["a"].(map[string]any)["$ref"])
	// An external pointer names a section of somebody else's document, which
	// this package has not renamed, so it must be left alone.
	require.Equal(t, "https://example.test/schema#/definitions/row", props["b"].(map[string]any)["$ref"])
}

func TestNormalizeConvertsDraft04ExclusiveBounds(t *testing.T) {
	out := normalized(t, `{
	  "$schema": "http://json-schema.org/draft-04/schema#",
	  "type": "object",
	  "id": "https://example.test/widget",
	  "properties": {
	    "open":   {"type": "number", "minimum": 0, "exclusiveMinimum": true},
	    "closed": {"type": "number", "minimum": 0, "exclusiveMinimum": false},
	    "capped": {"type": "number", "maximum": 10}
	  }
	}`)

	// "open": the exclusive bound replaces the inclusive one rather than sitting
	// beside it. "closed": a false flag leaves the inclusive bound in place.
	// "capped": an unflagged bound is untouched. And draft-04's id becomes $id.
	require.JSONEq(t, `{
	  "$schema": "`+Dialect202012+`",
	  "type": "object",
	  "$id": "https://example.test/widget",
	  "properties": {
	    "open":   {"type": "number", "exclusiveMinimum": 0},
	    "closed": {"type": "number", "minimum": 0},
	    "capped": {"type": "number", "maximum": 10}
	  }
	}`, mustJSON(t, out))
}

func TestNormalizeCarriesANumericDraft04ExclusiveBound(t *testing.T) {
	// A schema declaring draft-04 but spelling the bound the later way is
	// already in the 2020-12 form. Dropping it as "the boolean flag" would
	// silently remove the constraint.
	out := normalized(t, `{
	  "$schema": "http://json-schema.org/draft-04/schema#",
	  "type": "number",
	  "exclusiveMinimum": 5
	}`)
	require.JSONEq(t, `{"$schema": "`+Dialect202012+`", "type": "number", "exclusiveMinimum": 5}`, mustJSON(t, out))
}

func TestNormalizeLeavesSchemasItCannotClaimAlone(t *testing.T) {
	for name, doc := range map[string]string{
		"no dialect declared": `{"type": "object", "properties": {"a": {"type": "string"}}}`,
		"already 2020-12":     `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"unknown dialect":     `{"$schema": "https://example.test/dialect", "type": "object"}`,
		"draft 2019-09":       `{"$schema": "https://json-schema.org/draft/2019-09/schema", "type": "object"}`,
		// A tuple under no declared dialect is already malformed 2020-12.
		// Repairing it would mean guessing the author's dialect, which is
		// exactly the assumption this package refuses to make.
		"undeclared tuple": `{"type": "array", "items": [{"type": "string"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			in := parse(t, doc)
			out, res := Normalize(in)
			require.False(t, res.Changed)
			require.False(t, res.Skipped)
			require.Equal(t, in, out)
		})
	}
}

func TestNormalizeSkipsRatherThanGuessing(t *testing.T) {
	for name, tc := range map[string]struct {
		doc    string
		reason string
	}{
		"$ref with an assertion sibling": {
			doc:    `{"$schema": "` + draft7URI + `", "properties": {"a": {"$ref": "#/$defs/x", "maxLength": 3}}}`,
			reason: "maxLength",
		},
		"$ref nested under allOf": {
			doc:    `{"$schema": "` + draft7URI + `", "allOf": [{"$ref": "#/$defs/x", "minimum": 1}]}`,
			reason: "minimum",
		},
		"definitions and $defs together": {
			doc:    `{"$schema": "` + draft7URI + `", "definitions": {"a": {}}, "$defs": {"b": {}}}`,
			reason: "both definitions and $defs",
		},
		"tuple items and prefixItems together": {
			doc:    `{"$schema": "` + draft7URI + `", "items": [{}], "prefixItems": [{}]}`,
			reason: "both a draft-07 tuple items and prefixItems",
		},
		"dependencies and dependentRequired together": {
			doc:    `{"$schema": "` + draft7URI + `", "dependencies": {"a": ["b"]}, "dependentRequired": {"c": ["d"]}}`,
			reason: "both dependencies and dependentRequired",
		},
	} {
		t.Run(name, func(t *testing.T) {
			in := parse(t, tc.doc)
			out, res := Normalize(in)
			require.True(t, res.Skipped, "expected a skip")
			require.False(t, res.Changed)
			require.Contains(t, res.Reason, tc.reason)
			require.Equal(t, in, out, "a skipped schema must be returned verbatim")
		})
	}
}

// TestNormalizeIgnoresAnnotationSiblingsOfARef pins the other side of the
// skip rule: title and friends do not change what a schema accepts, so their
// presence beside a $ref must not cost the whole schema its translation.
func TestNormalizeIgnoresAnnotationSiblingsOfARef(t *testing.T) {
	out := normalized(t, `{
	  "$schema": "`+draft7URI+`",
	  "definitions": {"row": {"type": "string"}},
	  "properties": {"a": {"$ref": "#/definitions/row", "description": "a row", "title": "Row"}}
	}`)
	a := out["properties"].(map[string]any)["a"].(map[string]any)
	require.Equal(t, "#/$defs/row", a["$ref"])
	require.Equal(t, "a row", a["description"])
}

// TestNormalizeDoesNotRewriteDataValues guards against over-eager recursion.
// enum, const, default and examples hold instance data, which may itself be an
// object using the very key names this package rewrites in schema position.
func TestNormalizeDoesNotRewriteDataValues(t *testing.T) {
	payload := `{
	  "items": [{"type": "string"}],
	  "dependencies": {"a": ["b"]},
	  "definitions": {"x": {}},
	  "additionalItems": false
	}`
	out := normalized(t, `{
	  "$schema": "`+draft7URI+`",
	  "type": "object",
	  "default": `+payload+`,
	  "const": `+payload+`,
	  "examples": [`+payload+`],
	  "enum": [`+payload+`]
	}`)

	want := parse(t, payload)
	require.Equal(t, want, out["default"])
	require.Equal(t, want, out["const"])
	require.Equal(t, []any{want}, out["examples"])
	require.Equal(t, []any{want}, out["enum"])
}

// TestNormalizeTreatsPropertyNamesAsNames guards the same confusion from the
// other direction: a property may legitimately be called "$ref" or "items",
// and the properties map is a map of names, not a schema node.
func TestNormalizeTreatsPropertyNamesAsNames(t *testing.T) {
	out := normalized(t, `{
	  "$schema": "`+draft7URI+`",
	  "type": "object",
	  "properties": {
	    "$ref":  {"type": "string", "maxLength": 3},
	    "items": {"type": "array", "items": [{"type": "string"}]}
	  }
	}`)

	props := out["properties"].(map[string]any)
	require.Equal(t, map[string]any{"type": "string", "maxLength": float64(3)}, props["$ref"])
	// The nested tuple is still translated, so the traversal reached it.
	nested := props["items"].(map[string]any)
	require.Equal(t, []any{map[string]any{"type": "string"}}, nested["prefixItems"])
	require.NotContains(t, nested, "items")
}

// TestNormalizeDoesNotMutateItsInput matters because the schema handed to
// Normalize is shared by reference with the upstream client session's cached
// tool list, and discovery runs concurrently across servers.
func TestNormalizeDoesNotMutateItsInput(t *testing.T) {
	in := parse(t, `{
	  "$schema": "`+draft7URI+`",
	  "type": "object",
	  "definitions": {"row": {"type": "string"}},
	  "properties": {
	    "a": {"$ref": "#/definitions/row"},
	    "t": {"type": "array", "items": [{"type": "string"}], "additionalItems": false}
	  },
	  "dependencies": {"a": ["t"]}
	}`)
	before := parse(t, mustJSON(t, in))

	out, res := Normalize(in)
	require.True(t, res.Changed)

	require.Equal(t, before, in, "Normalize mutated its input")
	// And the copy must not alias the input, or a later write through one would
	// be visible in the other.
	outMap := out.(map[string]any)
	outMap["properties"].(map[string]any)["a"].(map[string]any)["$ref"] = "#/$defs/tampered"
	require.Equal(t, before, in)
}

func TestNormalizeIgnoresNonObjectSchemas(t *testing.T) {
	for name, in := range map[string]any{
		"nil":            nil,
		"boolean schema": true,
		"typed schema":   &jsonschema.Schema{Type: "object"},
		"string":         "not a schema",
	} {
		t.Run(name, func(t *testing.T) {
			out, res := Normalize(in)
			require.False(t, res.Changed)
			require.False(t, res.Skipped)
			require.Equal(t, in, out)
		})
	}
}

// TestNormalizePreservesWhatTheSchemaAccepts is the general property behind the
// per-keyword tests: for each translatable construct, the translated schema
// must reach the same verdict on the same instances as the original did. A
// shape assertion alone would pass a translation that renamed keywords into
// positions a validator ignores.
func TestNormalizePreservesWhatTheSchemaAccepts(t *testing.T) {
	for name, tc := range map[string]struct {
		doc       string
		instances []any
	}{
		"tuple": {
			doc: `{"$schema": "` + draft7URI + `", "type": "array",
			       "items": [{"type": "string"}, {"type": "number"}], "additionalItems": false}`,
			instances: []any{
				[]any{"a", 1.0},
				[]any{"a", "b"},
				[]any{"a", 1.0, "extra"},
				[]any{"a"},
			},
		},
		"dependencies": {
			doc: `{"$schema": "` + draft7URI + `", "type": "object",
			       "dependencies": {"creditCard": ["billingAddress"], "ship": {"required": ["addr"]}}}`,
			instances: []any{
				map[string]any{"creditCard": "x"},
				map[string]any{"creditCard": "x", "billingAddress": "y"},
				map[string]any{"ship": true},
				map[string]any{"ship": true, "addr": "z"},
				map[string]any{},
			},
		},
		"definitions and refs": {
			doc: `{"$schema": "` + draft7URI + `", "type": "object",
			       "definitions": {"row": {"type": "string"}},
			       "properties": {"a": {"$ref": "#/definitions/row"}}, "required": ["a"]}`,
			instances: []any{
				map[string]any{"a": "ok"},
				map[string]any{"a": 1.0},
				map[string]any{},
			},
		},
		// draft-04's boolean exclusive bounds have no equivalence case here on
		// purpose: google/jsonschema-go understands draft-07 and 2020-12 only,
		// so it cannot parse the "before" side and there is no oracle in this
		// repo to compare against. That translation is pinned by shape in
		// TestNormalizeConvertsDraft04ExclusiveBounds instead.
		"airtable list_bases": {
			doc: airtableListBasesOutputSchema,
			instances: []any{
				map[string]any{"bases": []any{map[string]any{"id": "app1", "name": "N", "permissionLevel": "create"}}},
				map[string]any{"bases": []any{map[string]any{"id": "app1"}}},
				map[string]any{"bases": []any{map[string]any{"id": "app1", "name": "N", "permissionLevel": "create", "extra": true}}},
				map[string]any{},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			original := parse(t, tc.doc)
			translated, res := Normalize(original)
			require.True(t, res.Changed)

			for _, instance := range tc.instances {
				wantErr := validate(t, original, instance)
				gotErr := validate(t, translated, instance)
				assert.Equal(t, wantErr == nil, gotErr == nil,
					"verdict changed for %#v: draft-07 err=%v, 2020-12 err=%v", instance, wantErr, gotErr)
			}
		})
	}
}

// validate resolves doc with google/jsonschema-go, which understands both
// draft-07 and 2020-12, and validates instance against it. A resolution failure
// is reported as a validation failure so a schema that stops compiling shows up
// as a changed verdict rather than a skipped assertion.
func validate(t *testing.T, doc any, instance any) error {
	t.Helper()
	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(mustJSON(t, doc)), &schema); err != nil {
		return err
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return err
	}
	return resolved.Validate(instance)
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}
