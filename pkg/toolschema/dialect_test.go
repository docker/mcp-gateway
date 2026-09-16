package toolschema

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const draft7URI = "http://json-schema.org/draft-07/schema#"

// reportedOutputSchema is the outputSchema from the report that motivated this
// package, captured verbatim from a tools/list round trip against
// @modelcontextprotocol/sdk 1.24.3 with zod 4. It is the shape that SDK emits
// for any zod-declared tool, so it stands in for a whole class of backend
// rather than for one product.
const reportedOutputSchema = `{
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
	out := normalized(t, reportedOutputSchema)
	require.Equal(t, Dialect202012, out["$schema"])

	// Nothing but the dialect should move on a schema this plain: the body is
	// already valid 2020-12 and only the declaration was making clients reject
	// it. Compare with $schema removed from both sides.
	in := parse(t, reportedOutputSchema)
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

// TestNormalizeKeepsDefinitionsAndRefsAsWritten pins a deliberate non-change.
//
// "definitions" was renamed to "$defs" in an earlier version of this package.
// It is not a keyword in 2020-12, but $ref resolution is JSON Pointer over the
// raw document and does not care, so both the section and pointers into it work
// untouched under a 2020-12 validator -- measured against ajv 8's 2020-12
// instance, in strict mode too. Renaming bought nothing and cost correctness:
// any rewrite of pointer text corrupts a pointer that legitimately traverses a
// property named "definitions", as the second assertion here shows.
//
// Subschemas inside the section are still translated, so the traversal has to
// reach them.
func TestNormalizeKeepsDefinitionsAndRefsAsWritten(t *testing.T) {
	out := normalized(t, `{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "type": "object",
	  "definitions": {
	    "row": {"type": "string"},
	    "pair": {"type": "array", "items": [{"type": "string"}], "additionalItems": false}
	  },
	  "properties": {
	    "definitions": {"type": "object"},
	    "a": {"$ref": "#/definitions/row"},
	    "b": {"$ref": "#/properties/definitions"},
	    "c": {"$ref": "#/properties/definitions/type"}
	  }
	}`)

	require.NotContains(t, out, "$defs")
	defs := out["definitions"].(map[string]any)
	require.Equal(t, map[string]any{"type": "string"}, defs["row"])
	// A tuple inside the section is still converted.
	require.Equal(t, []any{map[string]any{"type": "string"}}, defs["pair"].(map[string]any)["prefixItems"])

	props := out["properties"].(map[string]any)
	for name, want := range map[string]string{
		"a": "#/definitions/row",
		"b": "#/properties/definitions",
		"c": "#/properties/definitions/type",
	} {
		require.Equal(t, want, props[name].(map[string]any)["$ref"], "pointer %q was rewritten", name)
	}
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

// TestNormalizeRefusesANumericDraft04ExclusiveBound. draft-04 defines these
// keywords as BOOLEAN modifiers of minimum/maximum. A numeric value is the
// draft-06+ spelling, which draft-04 does not define, so it asserted nothing
// where the schema was written and would become a live bound the moment the
// document claims 2020-12.
//
// An earlier version of this package carried it across, for exactly the wrong
// reason: "it is already in the 2020-12 form". The form is right; the
// ACTIVATION is the problem, and it is the same mistake as any other keyword
// the source dialect does not define.
func TestNormalizeRefusesANumericDraft04ExclusiveBound(t *testing.T) {
	in := parse(t, `{
	  "$schema": "http://json-schema.org/draft-04/schema#",
	  "type": "number",
	  "exclusiveMinimum": 5
	}`)
	out, res := Normalize(in)
	require.True(t, res.Skipped)
	require.Contains(t, res.Reason, "non-boolean")
	require.Equal(t, in, out)
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
			doc:    `{"$schema": "` + draft7URI + `", "properties": {"a": {"$ref": "#/definitions/x", "maxLength": 3}}}`,
			reason: "maxLength",
		},
		"$ref nested under allOf": {
			doc:    `{"$schema": "` + draft7URI + `", "allOf": [{"$ref": "#/definitions/x", "minimum": 1}]}`,
			reason: "minimum",
		},
		"tuple items and prefixItems together": {
			doc:    `{"$schema": "` + draft7URI + `", "items": [{}], "prefixItems": [{}]}`,
			reason: "prefixItems",
		},
		"dependencies and dependentRequired together": {
			doc:    `{"$schema": "` + draft7URI + `", "dependencies": {"a": ["b"]}, "dependentRequired": {"c": ["d"]}}`,
			reason: "dependentRequired",
		},
		// Inert where the schema was written, live the moment it claims
		// 2020-12. Relabelling alone would start rejecting instances the
		// server's own validator accepted.
		"unevaluatedProperties": {
			doc:    `{"$schema": "` + draft7URI + `", "properties": {"a": {}}, "unevaluatedProperties": false}`,
			reason: "unevaluatedProperties",
		},
		"minContains": {
			doc:    `{"$schema": "` + draft7URI + `", "type": "array", "contains": {"type": "string"}, "minContains": 2}`,
			reason: "minContains",
		},
		"nested post-2020-12 keyword": {
			doc:    `{"$schema": "` + draft7URI + `", "properties": {"a": {"unevaluatedItems": false}}}`,
			reason: "unevaluatedItems",
		},
		// An embedded resource governed by its own declaration. Converting its
		// body under the root's rules while leaving that declaration alone
		// produces a document whose halves disagree.
		"embedded subschema declaring its own dialect": {
			doc:    `{"$schema": "` + draft7URI + `", "definitions": {"row": {"$schema": "` + draft7URI + `", "type": "array", "items": [{}]}}}`,
			reason: "embedded subschema declares its own $schema",
		},
		// Legal draft-07, forbidden in 2020-12, which spells it $anchor.
		// Rewriting it would have to repoint every $ref naming it.
		"plain-name fragment $id": {
			doc:    `{"$schema": "` + draft7URI + `", "definitions": {"row": {"$id": "#row", "type": "string"}}}`,
			reason: "URI fragment",
		},
		// Renaming id to $id would collide, and before this guard existed which
		// value survived depended on Go map iteration order.
		// draft-04 spells the identifier "id"; "$id" is an unknown keyword
		// there, and becomes the live identifier under 2020-12.
		"draft-04 $id": {
			doc:    `{"$schema": "http://json-schema.org/draft-04/schema#", "id": "https://a.test/x", "$id": "https://a.test/y"}`,
			reason: "$id",
		},
		// draft-04 requires the bound the flag modifies. With none, there is no
		// value to carry across and dropping the flag would widen the schema.
		"draft-04 exclusive flag with no bound": {
			doc:    `{"$schema": "http://json-schema.org/draft-04/schema#", "type": "number", "exclusiveMinimum": true}`,
			reason: "exclusiveMinimum with no minimum",
		},
		// Not the shape dependencies is defined to have, so it cannot be split
		// into the two keywords that replaced it. Dropping the key silently,
		// which is what an unguarded split did, is the worst available answer.
		"dependencies that is not an object": {
			doc:    `{"$schema": "` + draft7URI + `", "dependencies": "bogus"}`,
			reason: "not an object",
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
	require.Equal(t, "#/definitions/row", a["$ref"])
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

// TestNormalizeReturnsATypedSchemaUnchanged covers the gateway-built (POCI)
// shape, which is a *jsonschema.Schema rather than a decoded map and carries no
// dialect declaration to act on. Same, not Equal: the claim is that the very
// same value comes back, which an implementation returning an equal copy would
// satisfy under Equal.
func TestNormalizeReturnsATypedSchemaUnchanged(t *testing.T) {
	in := &jsonschema.Schema{Type: "object"}
	out, res := Normalize(in)
	require.False(t, res.Changed)
	require.False(t, res.Skipped)
	require.Same(t, in, out)
}

func TestNormalizeIgnoresNonObjectSchemas(t *testing.T) {
	for name, in := range map[string]any{
		"nil":            nil,
		"boolean schema": true,
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

// TestNormalizePreservesWhatTheSchemaAccepts checks the property behind the
// per-keyword tests: for each construct it covers, the translated schema must
// reach the same verdict on the same instances as the original did. A shape
// assertion alone would pass a translation that renamed keywords into positions
// a validator ignores.
//
// Its reach is limited by the oracle, and the limits are worth naming. It IS
// dialect-aware where it matters most -- measured: google/jsonschema-go enforces
// a draft-07 tuple items and dependencies, and ignores the byte-identical body
// under 2020-12 -- so these cases are not f(x) == f(x). It is NOT dialect-aware
// for keywords 2020-12 introduced (it enforces unevaluatedProperties even on a
// draft-07 document), so it structurally cannot witness that class. Those are
// refused outright instead, and pinned by TestNormalizeSkipsRatherThanGuessing.
// draft-04 is absent because the oracle cannot parse it at all.
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
		"$ref with annotation siblings": {
			doc: `{"$schema": "` + draft7URI + `", "type": "object",
			       "definitions": {"row": {"type": "string"}},
			       "properties": {"a": {"$ref": "#/definitions/row", "title": "Row", "description": "d"}},
			       "required": ["a"]}`,
			instances: []any{
				map[string]any{"a": "ok"},
				map[string]any{"a": 1.0},
				map[string]any{},
			},
		},
		"if then else": {
			doc: `{"$schema": "` + draft7URI + `", "type": "object",
			       "if": {"properties": {"kind": {"const": "a"}}, "required": ["kind"]},
			       "then": {"required": ["extra"]},
			       "else": {"required": ["other"]}}`,
			instances: []any{
				map[string]any{"kind": "a", "extra": 1.0},
				map[string]any{"kind": "a"},
				map[string]any{"kind": "b", "other": 1.0},
				map[string]any{"kind": "b"},
			},
		},
		"patternProperties and propertyNames": {
			doc: `{"$schema": "` + draft7URI + `", "type": "object",
			       "patternProperties": {"^x_": {"type": "number"}},
			       "propertyNames": {"maxLength": 4}}`,
			instances: []any{
				map[string]any{"x_a": 1.0},
				map[string]any{"x_a": "no"},
				map[string]any{"toolongname": 1.0},
			},
		},
		"boolean subschemas in a tuple": {
			doc: `{"$schema": "` + draft7URI + `", "type": "array",
			       "items": [true, false], "additionalItems": true}`,
			instances: []any{
				[]any{"anything"},
				[]any{"anything", "rejected-by-false"},
				[]any{},
			},
		},
		"the reported schema": {
			doc: reportedOutputSchema,
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

// TestNormalizeAcceptsEverySpellingOfEveryTranslatableDialect covers the
// dialect table itself. Before this test only one of the twelve keys was ever
// exercised: the http draft-07 spelling with a trailing "#". draft-06 support
// is claimed by the package doc and had no test at all.
func TestNormalizeAcceptsEverySpellingOfEveryTranslatableDialect(t *testing.T) {
	require.Len(t, translatableDialects, 12, "a new dialect spelling needs a case here")

	for declared := range translatableDialects {
		t.Run(declared, func(t *testing.T) {
			// A tuple, so the assertion needs the dialect to have been both
			// recognised and converted, not merely relabelled.
			out, res := Normalize(parse(t, `{
			  "$schema": "`+declared+`",
			  "type": "array",
			  "items": [{"type": "string"}],
			  "additionalItems": false
			}`))
			require.True(t, res.Changed, "dialect %q was not recognised", declared)
			require.False(t, res.Skipped, "unexpected skip: %s", res.Reason)
			require.JSONEq(t, `{
			  "$schema": "`+Dialect202012+`",
			  "type": "array",
			  "prefixItems": [{"type": "string"}],
			  "items": false
			}`, mustJSON(t, out))
		})
	}
}

// TestNormalizeDraft04BoundRewriteKeepsTheBoundary is the one rewrite in this
// package that silently changes the accept set if it is inverted, and
// google/jsonschema-go cannot read draft-04 so the equivalence harness is blind
// to it. The boundary value is the whole question: draft-04's
// minimum:0 + exclusiveMinimum:true rejects 0, and so must the translation.
func TestNormalizeDraft04BoundRewriteKeepsTheBoundary(t *testing.T) {
	out := normalized(t, `{
	  "$schema": "http://json-schema.org/draft-04/schema#",
	  "type": "number",
	  "minimum": 0,
	  "exclusiveMinimum": true
	}`)

	for _, tc := range []struct {
		instance float64
		accepted bool
	}{
		{instance: -1, accepted: false},
		{instance: 0, accepted: false}, // the boundary the exclusive flag excludes
		{instance: 1, accepted: true},
	} {
		err := validate(t, out, tc.instance)
		require.Equal(t, tc.accepted, err == nil, "instance %v: err=%v", tc.instance, err)
	}
}

// TestNormalizeDropsAdditionalItemsWithNoItems pins the third additionalItems
// case. draft-07 applies additionalItems only beside an array items, so with no
// items at all it constrains nothing and carrying it into 2020-12 -- where the
// keyword does not exist -- would be noise.
func TestNormalizeDropsAdditionalItemsWithNoItems(t *testing.T) {
	out := normalized(t, `{
	  "$schema": "`+draft7URI+`",
	  "type": "array",
	  "additionalItems": {"type": "string"}
	}`)
	require.JSONEq(t, `{"$schema": "`+Dialect202012+`", "type": "array"}`, mustJSON(t, out))
}

func TestNormalizeRefusesSchemasBeyondItsWalkBudget(t *testing.T) {
	// A tool schema is upstream-controlled input, so the traversal this package
	// added is bounded. Beyond the bound the schema is relayed as declared,
	// which is what the gateway did before this package existed.
	deep := `{"type": "object"}`
	for range maxSchemaDepth + 2 {
		deep = `{"properties": {"a": ` + deep + `}}`
	}
	in := parse(t, `{"$schema": "`+draft7URI+`", "properties": {"deep": `+deep+`}}`)
	out, res := Normalize(in)
	require.True(t, res.Skipped)
	require.Contains(t, res.Reason, "nests deeper")
	require.Equal(t, in, out)

	// Wide rather than deep, to show the node count is counted independently.
	wide := map[string]any{"$schema": draft7URI, "type": "object"}
	props := map[string]any{}
	for i := range maxSchemaNodes + 1 {
		props["p"+strconv.Itoa(i)] = map[string]any{"type": "string"}
	}
	wide["properties"] = props
	_, wideRes := Normalize(wide)
	require.True(t, wideRes.Skipped)
	require.Contains(t, wideRes.Reason, "more values")
}

// TestNormalizeGuardsListsInSingleSchemaPositions pins that the bail-out
// traversal dispatches on a value's type exactly as the converter does. A
// single-schema keyword holding a list is still rewritten, so a guard that
// stopped at the non-map would let a bail-out condition inside it through.
func TestNormalizeGuardsListsInSingleSchemaPositions(t *testing.T) {
	in := parse(t, `{
	  "$schema": "`+draft7URI+`",
	  "not": [{"$ref": "#/definitions/row", "maxLength": 3}],
	  "definitions": {"row": {"type": "string"}}
	}`)
	out, res := Normalize(in)
	require.True(t, res.Skipped, "a $ref sibling inside a list position was not guarded")
	require.Contains(t, res.Reason, "maxLength")
	require.Equal(t, in, out)
}

// TestNormalizeReportsADialectItCannotTranslate closes the gap between "there
// was nothing to do" and "this schema is affected and beyond our reach". Both
// come back unchanged, but only the second is a thing an operator sizing the
// affected surface needs to see: a draft-03 or 2019-09 schema is still rejected
// by the same 2020-12-only clients this package exists for.
func TestNormalizeReportsADialectItCannotTranslate(t *testing.T) {
	for name, tc := range map[string]struct {
		declared    string
		unsupported bool
	}{
		"draft-03":           {declared: "http://json-schema.org/draft-03/schema#", unsupported: true},
		"2019-09":            {declared: "https://json-schema.org/draft/2019-09/schema", unsupported: true},
		"nonsense":           {declared: "https://example.test/dialect", unsupported: true},
		"2020-12":            {declared: Dialect202012, unsupported: false},
		"2020-12 with a '#'": {declared: Dialect202012 + "#", unsupported: false},
	} {
		t.Run(name, func(t *testing.T) {
			in := parse(t, `{"$schema": "`+tc.declared+`", "type": "object"}`)
			out, res := Normalize(in)
			require.False(t, res.Changed)
			require.False(t, res.Skipped)
			require.Equal(t, tc.unsupported, res.UnsupportedDialect)
			require.Equal(t, in, out)
		})
	}

	// A schema declaring nothing is 2020-12 by default, so there is genuinely
	// nothing to report about it.
	_, res := Normalize(parse(t, `{"type": "object"}`))
	require.False(t, res.UnsupportedDialect)
	require.False(t, res.Changed)
}

// TestNormalizeRefusesKeywordsTheSourceDialectDoesNotDefine is the case an
// earlier shared blacklist could not express, and the reason the guard is now
// keyed to the declared dialect.
//
// Each of these is a legal, inert, ignored extension in the dialect it is
// written in, and an assertion under 2020-12. The draft-04 `const` row is the
// sharpest: relabelling turns "accepts every instance" into "accepts only x".
// The keyword sets come from each draft's own meta-schema, so this table is
// also the regression test for getting one of those sets wrong.
func TestNormalizeRefusesKeywordsTheSourceDialectDoesNotDefine(t *testing.T) {
	const (
		d4 = "http://json-schema.org/draft-04/schema#"
		d6 = "http://json-schema.org/draft-06/schema#"
		d7 = "http://json-schema.org/draft-07/schema#"
	)

	for name, tc := range map[string]struct {
		dialect string
		body    string
		keyword string
	}{
		// draft-04 defines none of these; draft-06 added them.
		"draft-04 const":         {dialect: d4, body: `"const": "x"`, keyword: "const"},
		"draft-04 contains":      {dialect: d4, body: `"contains": {"type": "string"}`, keyword: "contains"},
		"draft-04 propertyNames": {dialect: d4, body: `"propertyNames": {"maxLength": 2}`, keyword: "propertyNames"},
		// draft-04 and draft-06 define none of these; draft-07 added them.
		"draft-04 if":   {dialect: d4, body: `"if": {"type": "string"}`, keyword: "if"},
		"draft-06 if":   {dialect: d6, body: `"if": {"type": "string"}`, keyword: "if"},
		"draft-06 then": {dialect: d6, body: `"then": {"type": "string"}`, keyword: "then"},
		"draft-06 else": {dialect: d6, body: `"else": {"type": "string"}`, keyword: "else"},
		// 2019-09 and later.
		"draft-07 unevaluatedProperties": {dialect: d7, body: `"unevaluatedProperties": false`, keyword: "unevaluatedProperties"},
		"draft-07 contentSchema":         {dialect: d7, body: `"contentSchema": {"type": "string"}`, keyword: "contentSchema"},
		"draft-07 $anchor":               {dialect: d7, body: `"$anchor": "row"`, keyword: "$anchor"},
		"draft-06 $comment is not":       {dialect: d6, body: `"$comment": "inert either way"`, keyword: ""},
		// Annotations cannot change an outcome, so they must NOT cost a bail.
		"draft-04 readOnly is not":   {dialect: d4, body: `"readOnly": true`, keyword: ""},
		"draft-04 examples is not":   {dialect: d4, body: `"examples": [1]`, keyword: ""},
		"draft-04 $defs is not":      {dialect: d4, body: `"$defs": {"a": {"type": "string"}}`, keyword: ""},
		"draft-06 deprecated is not": {dialect: d6, body: `"deprecated": true`, keyword: ""},
	} {
		t.Run(name, func(t *testing.T) {
			in := parse(t, `{"$schema": "`+tc.dialect+`", "type": "object", `+tc.body+`}`)
			out, res := Normalize(in)

			if tc.keyword == "" {
				require.False(t, res.Skipped,
					"an annotation must not cost the schema its translation: %s", res.Reason)
				require.True(t, res.Changed)
				return
			}
			require.True(t, res.Skipped, "expected a refusal for %q", tc.keyword)
			require.Contains(t, res.Reason, tc.keyword)
			require.Equal(t, in, out)
		})
	}
}

// TestNormalizeTranslatesASchemaWhoseRootIsARef. The root dialect declaration
// is not a $ref sibling in the sense that matters -- it is how the schema says
// which dialect it is written in, and every schema this package acts on has
// one. Treating it as an unsafe sibling made a root-$ref schema permanently
// untranslatable, which is the opposite of the intent.
func TestNormalizeTranslatesASchemaWhoseRootIsARef(t *testing.T) {
	out := normalized(t, `{
	  "$schema": "`+draft7URI+`",
	  "$ref": "#/definitions/row",
	  "definitions": {"row": {"type": "array", "items": [{"type": "string"}]}}
	}`)

	require.Equal(t, Dialect202012, out["$schema"])
	require.Equal(t, "#/definitions/row", out["$ref"])
	// And the subschema it points at was still translated.
	row := out["definitions"].(map[string]any)["row"].(map[string]any)
	require.Equal(t, []any{map[string]any{"type": "string"}}, row["prefixItems"])

	// A NESTED $schema beside a $ref is still refused.
	nested := parse(t, `{
	  "$schema": "`+draft7URI+`",
	  "properties": {"a": {"$schema": "`+draft7URI+`", "$ref": "#/definitions/row"}}
	}`)
	_, res := Normalize(nested)
	require.True(t, res.Skipped)
	require.Contains(t, res.Reason, "embedded subschema declares its own $schema")
}

// TestNormalizeRefusesEveryIdentifierFragmentForm. 2020-12 forbids any
// non-empty fragment in an identifier. An earlier version checked only for a
// leading "#name", which let "#/row" and an absolute URI ending in "#row"
// through into a translated $id that 2020-12 rejects outright.
func TestNormalizeRefusesEveryIdentifierFragmentForm(t *testing.T) {
	for name, tc := range map[string]struct{ dialect, id string }{
		"draft-07 plain name": {dialect: draft7URI, id: "#row"},
		"draft-07 pointer":    {dialect: draft7URI, id: "#/row"},
		"draft-07 absolute":   {dialect: draft7URI, id: "https://example.test/s#row"},
		"draft-07 bare hash":  {dialect: draft7URI, id: "#"},
		"draft-04 plain name": {dialect: "http://json-schema.org/draft-04/schema#", id: "#row"},
		"draft-04 absolute":   {dialect: "http://json-schema.org/draft-04/schema#", id: "https://example.test/s#row"},
	} {
		t.Run(name, func(t *testing.T) {
			key := "$id"
			if tc.dialect == "http://json-schema.org/draft-04/schema#" {
				key = "id"
			}
			in := parse(t, `{"$schema": "`+tc.dialect+`", "`+key+`": "`+tc.id+`", "type": "object"}`)
			out, res := Normalize(in)
			require.True(t, res.Skipped, "identifier %q was not refused", tc.id)
			require.Contains(t, res.Reason, "URI fragment")
			require.Equal(t, in, out)
		})
	}

	// An identifier with no fragment is fine and must still translate.
	out := normalized(t, `{"$schema": "`+draft7URI+`", "$id": "https://example.test/s", "type": "object"}`)
	require.Equal(t, "https://example.test/s", out["$id"])
}
