package gcf

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// GenericSet is a keyed record set: the unit generic-profile delta operates on
// (SPEC Section 10a). Rows are order-agnostic (set semantics); Fields carries the
// declared column order for the wire form; Key names the identity column (the
// `@id` / `key=`); Name is the tabular section name for a full payload.
//
// Delta is defined over an explicit, fixed schema — not arbitrary `any` — because
// identity, uniqueness, and schema-stability are all required (Section 10a.1).
type GenericSet struct {
	Name   string
	Key    string
	Fields []string
	Rows   []map[string]any
}

// GenericDeltaPayload is a diff between two GenericSets (computed by
// DiffGenericSets, or supplied directly and serialized by EncodeGenericDelta).
type GenericDeltaPayload struct {
	Tool        string
	Key         string
	Fields      []string
	BaseRoot    string
	NewRoot     string
	Added       []map[string]any
	Changed     []map[string]any
	Removed     []any // identity values
	DeltaTokens int
	FullTokens  int
}

// canonicalCell canonicalizes one value for the pack-root record (Section 10a.3).
// Purpose-built and deliberately decoupled from the wire cell encoder
// (formatScalar): it must be collision-free and record-safe, not round-trippable.
//   - Typed literals stay bare so they never collide with the strings that spell
//     them: null is "-" (never a string), booleans are true/false, numbers are
//     canonical (Section 2.3.1).
//   - Strings are ALWAYS quoted, so (a) they can't collide with a typed literal
//     ("-", "true", "123" all become quoted), and (b) a tab or newline inside a
//     value is escaped and cannot break the tab/newline-delimited record.
func canonicalCell(v any) string {
	switch val := v.(type) {
	case nil:
		return "-"
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		return formatNumber(val)
	case int:
		return formatNumber(float64(val))
	case int64:
		return formatNumber(float64(val))
	case string:
		return quoteString(val)
	default:
		return quoteString(fmt.Sprintf("%v", val))
	}
}

// GenericPackRoot computes the canonical pack root for a keyed set using the
// gcf-pack-root-v1 algorithm, generic profile (SPEC Section 10a.3). Two
// implementations given the same logical set MUST produce the same result.
func GenericPackRoot(s GenericSet) string {
	sortedFields := append([]string(nil), s.Fields...)
	sort.Strings(sortedFields)

	records := make([]string, len(s.Rows))
	for i, row := range s.Rows {
		var b strings.Builder
		b.WriteByte('R')
		for _, f := range sortedFields {
			b.WriteByte('\t')
			b.WriteString(f)
			b.WriteByte('\t')
			b.WriteString(canonicalCell(row[f]))
		}
		b.WriteByte('\n')
		records[i] = b.String()
	}
	sort.Strings(records)

	var b strings.Builder
	for _, r := range records {
		b.WriteString(r)
	}
	h := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("sha256:%x", h)
}

// indexByKey builds an identity -> row index, rejecting duplicate identities
// (delta is undefined without unique identity; Section 10a.1).
func indexByKey(s GenericSet) (map[string]map[string]any, error) {
	m := make(map[string]map[string]any, len(s.Rows))
	for _, row := range s.Rows {
		id := canonicalCell(row[s.Key])
		if _, dup := m[id]; dup {
			return nil, fmt.Errorf("delta_invalid: duplicate identity %s for key %q", id, s.Key)
		}
		m[id] = row
	}
	return m, nil
}

// DiffGenericSets computes the delta from base to next. This is the blessed
// producer path: it is the single place that enforces the keyed-diff invariants
// (identity uniqueness, added-not-in-base, changed-must-exist, whole-row
// replacement, unchanged rows omitted). Added/Changed/Removed are sorted by
// identity for reproducible output (Section 10a.6). Schema change or a missing
// key returns an error — the caller must then send a full payload (Section 10a.7).
func DiffGenericSets(base, next GenericSet) (*GenericDeltaPayload, error) {
	if next.Key == "" {
		return nil, fmt.Errorf("delta_invalid: no identity key")
	}
	if next.Key != base.Key || !sameStrings(base.Fields, next.Fields) {
		return nil, fmt.Errorf("delta_invalid: schema change (send full)")
	}
	baseByID, err := indexByKey(base)
	if err != nil {
		return nil, err
	}
	nextByID, err := indexByKey(next)
	if err != nil {
		return nil, err
	}

	d := &GenericDeltaPayload{
		Key:      next.Key,
		Fields:   append([]string(nil), next.Fields...),
		BaseRoot: GenericPackRoot(base),
		NewRoot:  GenericPackRoot(next),
	}
	for id, row := range nextByID {
		if brow, ok := baseByID[id]; !ok {
			d.Added = append(d.Added, row)
		} else if !rowsEqual(brow, row, next.Fields) {
			d.Changed = append(d.Changed, row)
		}
		// equal rows are omitted (silence = "keep it", Section 10a.5)
	}
	for id, brow := range baseByID {
		if _, ok := nextByID[id]; !ok {
			d.Removed = append(d.Removed, brow[next.Key])
		}
	}
	sortRowsByKey(d.Added, d.Key)
	sortRowsByKey(d.Changed, d.Key)
	sort.Slice(d.Removed, func(i, j int) bool {
		return canonicalCell(d.Removed[i]) < canonicalCell(d.Removed[j])
	})
	return d, nil
}

// EncodeGenericFull emits a delta-participating full base payload: `key=` in the
// header, an `@`-prefixed identity field in the declaration, pipe-separated rows.
func EncodeGenericFull(s GenericSet, tool string) string {
	name := s.Name
	if name == "" {
		name = "rows"
	}
	var b strings.Builder
	b.WriteString("GCF profile=generic")
	if tool != "" {
		b.WriteString(" tool=" + tool)
	}
	b.WriteString(" pack_root=" + GenericPackRoot(s) + " key=" + s.Key + "\n")
	b.WriteString(fmt.Sprintf("## %s [%d]{%s}\n", name, len(s.Rows), fieldDecl(s.Fields, s.Key)))
	for _, row := range s.Rows {
		b.WriteString(encodeRow(row, s.Fields))
		b.WriteByte('\n')
	}
	return b.String()
}

// EncodeGenericDelta serializes a delta payload (SPEC Section 10a.2). Mirrors the
// graph EncodeDelta; sections are emitted in the deterministic order
// added / changed / removed (Section 10a.6).
func EncodeGenericDelta(d *GenericDeltaPayload) string {
	var b strings.Builder
	b.WriteString("GCF profile=generic")
	if d.Tool != "" {
		b.WriteString(" tool=" + d.Tool)
	}
	b.WriteString(" delta=true base_root=" + d.BaseRoot + " new_root=" + d.NewRoot + " key=" + d.Key)
	if d.FullTokens > 0 {
		savings := 100.0 * (1.0 - float64(d.DeltaTokens)/float64(d.FullTokens))
		b.WriteString(fmt.Sprintf(" savings=%.0f%%", savings))
	}
	b.WriteByte('\n')

	if len(d.Added) > 0 {
		b.WriteString(fmt.Sprintf("## added [%d]{%s}\n", len(d.Added), fieldDecl(d.Fields, d.Key)))
		for _, row := range d.Added {
			b.WriteString(encodeRow(row, d.Fields))
			b.WriteByte('\n')
		}
	}
	if len(d.Changed) > 0 {
		b.WriteString(fmt.Sprintf("## changed [%d]{%s}\n", len(d.Changed), fieldDecl(d.Fields, d.Key)))
		for _, row := range d.Changed {
			b.WriteString(encodeRow(row, d.Fields))
			b.WriteByte('\n')
		}
	}
	if len(d.Removed) > 0 {
		b.WriteString(fmt.Sprintf("## removed [%d]{@%s}\n", len(d.Removed), d.Key))
		for _, idv := range d.Removed {
			b.WriteString(formatScalar(idv, '|'))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// VerifyGenericDelta applies a delta to a base set and verifies the result hashes
// to expectedNewRoot (SPEC Section 10a.5). Atomic: the whole payload is validated
// before any state changes, and a mismatch leaves the base untouched. This is the
// consumer-side correctness core — it proves the algorithm end to end without the
// wire parser.
func VerifyGenericDelta(base GenericSet, d *GenericDeltaPayload, expectedNewRoot string) (GenericSet, error) {
	if GenericPackRoot(base) != d.BaseRoot {
		return GenericSet{}, fmt.Errorf("base_mismatch: base root does not equal delta base_root")
	}
	baseByID, err := indexByKey(base)
	if err != nil {
		return GenericSet{}, err
	}

	// Validate the entire payload against the original base before mutating.
	for _, idv := range d.Removed {
		if _, ok := baseByID[canonicalCell(idv)]; !ok {
			return GenericSet{}, fmt.Errorf("delta_invalid: removing identity %s not in base", canonicalCell(idv))
		}
	}
	for _, row := range d.Added {
		if _, ok := baseByID[canonicalCell(row[d.Key])]; ok {
			return GenericSet{}, fmt.Errorf("delta_invalid: adding identity %s that already exists", canonicalCell(row[d.Key]))
		}
	}
	for _, row := range d.Changed {
		if _, ok := baseByID[canonicalCell(row[d.Key])]; !ok {
			return GenericSet{}, fmt.Errorf("delta_invalid: changing identity %s not in base", canonicalCell(row[d.Key]))
		}
	}

	// Apply to a working copy.
	work := make(map[string]map[string]any, len(baseByID))
	for id, row := range baseByID {
		work[id] = row
	}
	for _, idv := range d.Removed {
		delete(work, canonicalCell(idv))
	}
	for _, row := range d.Added {
		work[canonicalCell(row[d.Key])] = row
	}
	for _, row := range d.Changed {
		work[canonicalCell(row[d.Key])] = row
	}

	result := GenericSet{Name: base.Name, Key: base.Key, Fields: base.Fields, Rows: make([]map[string]any, 0, len(work))}
	for _, row := range work {
		result.Rows = append(result.Rows, row)
	}
	if got := GenericPackRoot(result); got != expectedNewRoot {
		return GenericSet{}, fmt.Errorf("root_mismatch: computed %s, expected %s", got, expectedNewRoot)
	}
	return result, nil
}

// --- helpers ---

func fieldDecl(fields []string, key string) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		if f == key {
			parts[i] = "@" + formatKey(f)
		} else {
			parts[i] = formatKey(f)
		}
	}
	return strings.Join(parts, ",")
}

func encodeRow(row map[string]any, fields []string) string {
	cells := make([]string, len(fields))
	for i, f := range fields {
		cells[i] = formatScalar(row[f], '|')
	}
	return strings.Join(cells, "|")
}

func rowsEqual(a, b map[string]any, fields []string) bool {
	for _, f := range fields {
		if canonicalCell(a[f]) != canonicalCell(b[f]) {
			return false
		}
	}
	return true
}

func sameStrings(a, b []string) bool {
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

func sortRowsByKey(rows []map[string]any, key string) {
	sort.Slice(rows, func(i, j int) bool {
		return canonicalCell(rows[i][key]) < canonicalCell(rows[j][key])
	})
}

// --- consumer-side wire parsing (SPEC Section 10a) ---

// parseHeaderFields splits a GCF header line into its k=v tokens. Values are
// space-free (roots, key, tool), so whitespace tokenization is sufficient.
func parseHeaderFields(header string) map[string]string {
	m := map[string]string{}
	for _, tok := range strings.Fields(header) {
		if i := strings.IndexByte(tok, '='); i > 0 {
			m[tok[:i]] = tok[i+1:]
		}
	}
	return m
}

// splitDeltaFieldDecl parses a delta/full field declaration `{@id,total,...}`.
// Unlike splitFieldDecl it accepts the `@`-prefixed identity field (Section 10a.1),
// returning the ordered fields and the key field (the one that was `@`-marked).
func splitDeltaFieldDecl(decl string) (fields []string, keyField string, err error) {
	if len(decl) < 2 || decl[0] != '{' || decl[len(decl)-1] != '}' {
		return nil, "", fmt.Errorf("invalid field declaration: %s", decl)
	}
	inner := decl[1 : len(decl)-1]
	if inner == "" {
		return nil, "", nil
	}
	for _, raw := range splitRespectingQuotes(inner, ',') {
		f := strings.TrimSpace(raw)
		isKey := false
		if strings.HasPrefix(f, "@") {
			f = f[1:]
			isKey = true
		}
		if len(f) >= 2 && f[0] == '"' && f[len(f)-1] == '"' {
			p, perr := parseQuotedString(f)
			if perr != nil {
				return nil, "", perr
			}
			f = p
		}
		if isKey {
			keyField = f
		}
		fields = append(fields, f)
	}
	return fields, keyField, nil
}

// parseSectionHeader parses the content after `## ` of a delta/full section, e.g.
// `added [1]{@id,total,status,customer}` or `orders [3]{@id,...}` or `removed [1]{@id}`.
func parseSectionHeader(content string) (name string, count int, fields []string, keyField string, err error) {
	bi := findBracketStart(content)
	if bi < 0 {
		return "", 0, nil, "", fmt.Errorf("delta_invalid: section header without count: %q", content)
	}
	name = strings.TrimSpace(content[:bi])
	rest := content[bi+1:] // "[N]{...}"
	if len(rest) == 0 || rest[0] != '[' {
		return "", 0, nil, "", fmt.Errorf("delta_invalid: malformed section header: %q", content)
	}
	close := strings.IndexByte(rest, ']')
	if close < 0 {
		return "", 0, nil, "", fmt.Errorf("delta_invalid: unterminated count: %q", content)
	}
	count, err = parseCount(rest[1:close])
	if err != nil {
		return "", 0, nil, "", err
	}
	fields, keyField, err = splitDeltaFieldDecl(rest[close+1:])
	return name, count, fields, keyField, err
}

func parseRow(line string, fields []string) (map[string]any, error) {
	cells := splitRespectingQuotes(line, '|')
	if len(cells) != len(fields) {
		return nil, fmt.Errorf("delta_invalid: row has %d cells, expected %d: %q", len(cells), len(fields), line)
	}
	row := make(map[string]any, len(fields))
	for i, f := range fields {
		v, err := parseScalar(cells[i], true)
		if err != nil {
			return nil, err
		}
		row[f] = v
	}
	return row, nil
}

// DecodeGenericFull parses a delta-participating full base payload into a
// GenericSet, and returns the declared pack_root (Section 10a).
func DecodeGenericFull(text string) (GenericSet, string, error) {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) == 0 {
		return GenericSet{}, "", fmt.Errorf("empty payload")
	}
	hdr := parseHeaderFields(lines[0])
	if hdr["profile"] != "generic" {
		return GenericSet{}, "", fmt.Errorf("not a generic payload")
	}
	s := GenericSet{Key: hdr["key"]}
	for i := 1; i < len(lines); {
		line := lines[i]
		if !strings.HasPrefix(line, "## ") {
			// Only blank lines, comments, and the ##! summary trailer are valid
			// outside a section; any other line is a surplus row past a declared
			// section count (Section 13).
			if line == "" || strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "##! ") {
				i++
				continue
			}
			return GenericSet{}, "", fmt.Errorf("count_mismatch: unexpected content after declared section rows: %q", line)
		}
		name, count, fields, keyField, err := parseSectionHeader(line[3:])
		if err != nil {
			return GenericSet{}, "", err
		}
		s.Name, s.Fields = name, fields
		if s.Key == "" {
			s.Key = keyField
		}
		i++
		for j := 0; j < count; j++ {
			if i >= len(lines) || strings.HasPrefix(lines[i], "## ") {
				return GenericSet{}, "", fmt.Errorf("count_mismatch: declared %d rows, got %d", count, j)
			}
			row, err := parseRow(lines[i], fields)
			if err != nil {
				return GenericSet{}, "", err
			}
			s.Rows = append(s.Rows, row)
			i++
		}
	}
	return s, hdr["pack_root"], nil
}

// DecodeGenericDelta parses a delta payload into a GenericDeltaPayload
// (Section 10a.2). The result can be applied with VerifyGenericDelta.
func DecodeGenericDelta(text string) (*GenericDeltaPayload, error) {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	hdr := parseHeaderFields(lines[0])
	if hdr["profile"] != "generic" {
		return nil, fmt.Errorf("not a generic payload")
	}
	if hdr["delta"] != "true" {
		return nil, fmt.Errorf("not a delta payload")
	}
	d := &GenericDeltaPayload{
		Tool: hdr["tool"], Key: hdr["key"],
		BaseRoot: hdr["base_root"], NewRoot: hdr["new_root"],
	}
	for i := 1; i < len(lines); {
		line := lines[i]
		if !strings.HasPrefix(line, "## ") {
			// Only blank lines, comments, and the ##! summary trailer are valid
			// outside a section; any other line is a surplus row past a declared
			// section count (Section 13).
			if line == "" || strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "##! ") {
				i++
				continue
			}
			return nil, fmt.Errorf("count_mismatch: unexpected content after declared section rows: %q", line)
		}
		name, count, fields, keyField, err := parseSectionHeader(line[3:])
		if err != nil {
			return nil, err
		}
		if d.Key == "" && keyField != "" {
			d.Key = keyField
		}
		if d.Fields == nil && (name == "added" || name == "changed") {
			d.Fields = fields
		}
		i++
		switch name {
		case "added", "changed":
			rows := make([]map[string]any, 0, count)
			for j := 0; j < count; j++ {
				if i >= len(lines) || strings.HasPrefix(lines[i], "## ") {
					return nil, fmt.Errorf("count_mismatch: declared %d rows in ## %s, got %d", count, name, j)
				}
				row, err := parseRow(lines[i], fields)
				if err != nil {
					return nil, err
				}
				rows = append(rows, row)
				i++
			}
			if name == "added" {
				d.Added = rows
			} else {
				d.Changed = rows
			}
		case "removed":
			for j := 0; j < count; j++ {
				if i >= len(lines) || strings.HasPrefix(lines[i], "## ") {
					return nil, fmt.Errorf("count_mismatch: declared %d identities in ## removed, got %d", count, j)
				}
				v, err := parseScalar(lines[i], true)
				if err != nil {
					return nil, err
				}
				d.Removed = append(d.Removed, v)
				i++
			}
		default:
			return nil, fmt.Errorf("delta_invalid: unknown delta section %q", name)
		}
	}
	return d, nil
}
