package gcf

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// jsonNumberRe matches the JSON number grammar exactly.
var jsonNumberRe = regexp.MustCompile(`^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?$`)

// numericLikeRe matches tokens that are numeric-like per Section 2.4:
// after an optional leading + or -, begins with a digit, or begins with . followed by a digit.
var numericLikeRe = regexp.MustCompile(`^[+-]\.?\d|^\.\d|^0\d`)
var inlineArrayRe = regexp.MustCompile(`\[[^\]]*\]\s*:`)

// needsQuote returns true if a string value must be quoted per Section 2.4.
func needsQuote(s string) bool {
	if s == "" {
		return true
	}
	if s == "-" || s == "~" || s == "^" || s == "true" || s == "false" {
		return true
	}
	// A value shaped like an inline-schema attachment marker (^{...}) would decode
	// as an attachment and lose the string, so it must be quoted (SPEC 2.4).
	if len(s) >= 3 && s[0] == '^' && s[1] == '{' && s[len(s)-1] == '}' {
		return true
	}
	if jsonNumberRe.MatchString(s) {
		return true
	}
	if numericLikeRe.MatchString(s) {
		return true
	}
	if s[0] == ' ' || s[len(s)-1] == ' ' {
		return true
	}
	if s[0] == '#' || s[0] == '@' || s[0] == '.' {
		return true
	}
	if inlineArrayRe.MatchString(s) {
		return true
	}
	for _, c := range s {
		if c == '"' || c == '\\' || c == '|' || c == ',' || c < 0x20 || c == '\n' || c == '\r' {
			return true
		}
		// C1 controls (U+0080-U+009F).
		if c >= 0x80 && c <= 0x9F {
			return true
		}
		// Unicode whitespace (U+00A0 NBSP, U+1680, U+2000-U+200A, U+2028, U+2029, U+202F, U+205F, U+3000, U+FEFF).
		if c > 0x7F && unicode.IsSpace(c) {
			return true
		}
		if c == 0xFEFF {
			return true
		}
	}
	return false
}

// needsQuoteInContext checks quoting for a specific delimiter context.
func needsQuoteInContext(s string, delimiter byte) bool {
	if needsQuote(s) {
		return true
	}
	if delimiter != 0 && strings.ContainsRune(s, rune(delimiter)) {
		return true
	}
	return false
}

// quoteString produces a JSON-compatible quoted string with proper escaping.
func quoteString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// formatScalar formats a Go value as a GCF scalar.
// delimiter is the context delimiter: '|' for tabular, ',' for inline arrays, 0 for kv-line.
func formatScalar(v any, delimiter byte) string {
	if v == nil {
		return "-"
	}
	switch val := v.(type) {
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		return formatNumber(val)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case string:
		if needsQuoteInContext(val, delimiter) {
			return quoteString(val)
		}
		return val
	default:
		s := fmt.Sprintf("%v", val)
		if needsQuoteInContext(s, delimiter) {
			return quoteString(s)
		}
		return s
	}
}

// formatNumber formats a float64 per Section 2.3 canonical rules.
func formatNumber(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		// Should not happen for JSON values; reject per spec.
		return "0"
	}
	if f == 0 {
		// Negative zero canonicalizes to 0 (SPEC 2.3.1): -0.0 equals 0.0 by value.
		return "0"
	}
	abs := math.Abs(f)
	// Plain decimal only below 2^53. Every double at or above 2^53 is integer-valued
	// (a binary64 has no fractional part past 2^52), so a plain rendering would emit a
	// bare-integer token: indistinguishable from an int64 on the wire, and beyond the
	// binary64 safe-integer range (2^53-1) so a JavaScript decoder rejects it under its
	// default policy. Rendering such doubles in exponent shape keeps bare tokens
	// unambiguously int64 and decimal/exponent tokens doubles (SPEC 2.3.1).
	// 9007199254740992.0 is 2^53.
	if abs >= 1e-6 && abs < 9007199254740992.0 {
		// Plain decimal. Use strconv to get exact representation.
		s := strconv.FormatFloat(f, 'f', -1, 64)
		return s
	}
	// Exponent notation: normalized, lowercase e, explicit sign, no leading exponent zeroes.
	s := strconv.FormatFloat(f, 'e', -1, 64)
	// Go emits e+06 or e-07; strip leading zeros from exponent.
	if idx := strings.IndexByte(s, 'e'); idx >= 0 {
		expPart := s[idx+1:]
		sign := ""
		digits := expPart
		if expPart[0] == '+' || expPart[0] == '-' {
			sign = string(expPart[0])
			digits = expPart[1:]
		}
		// Strip leading zeros.
		stripped := strings.TrimLeft(digits, "0")
		if stripped == "" {
			stripped = "0"
		}
		s = s[:idx+1] + sign + stripped
	}
	return s
}

// isBareKey returns true if s is a valid bare key (Section 2a.1).
func isBareKey(s string) bool {
	if len(s) == 0 {
		return false
	}
	c := s[0]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
		return false
	}
	for i := 1; i < len(s); i++ {
		c = s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// formatKey formats a key, quoting if necessary.
func formatKey(s string) string {
	if isBareKey(s) {
		return s
	}
	return quoteString(s)
}

// parseScalar parses a GCF scalar token per Section 2.1 precedence.
// tabularContext should be true when parsing tabular row cells.
func parseScalar(s string, tabularContext bool) (any, error) {
	if s == "" {
		return "", nil
	}

	// 1. Quoted string.
	if s[0] == '"' {
		return parseQuotedString(s)
	}

	// 2. Null.
	if s == "-" {
		return nil, nil
	}

	// 3. Missing (tabular only).
	if s == "~" {
		if !tabularContext {
			return nil, fmt.Errorf("invalid_missing: ~ outside tabular row cell")
		}
		return missingMarker{}, nil
	}

	// 4. Attachment (tabular only).
	if s == "^" {
		if !tabularContext {
			return nil, fmt.Errorf("invalid_attachment_marker: ^ outside tabular row cell")
		}
		return attachmentMarker{}, nil
	}

	// 5. Boolean.
	if s == "true" {
		return true, nil
	}
	if s == "false" {
		return false, nil
	}

	// 6. Number (JSON number grammar).
	if jsonNumberRe.MatchString(s) {
		// A plain integer literal (no fraction, no exponent) is parsed directly to
		// int64 so the full signed 64-bit domain round-trips exactly. Routing it
		// through a float64 would silently approximate any magnitude beyond 2^53
		// (SPEC 2.3.2). Fraction/exponent forms remain IEEE-754 doubles.
		if !strings.Contains(s, ".") && !strings.ContainsAny(s, "eE") {
			// A bare token (no fraction, no exponent) is an integer; `-0` is integer
			// syntax and decodes to the value zero (SPEC 2.3.1), not a float -0.0.
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				if errors.Is(err, strconv.ErrRange) {
					return nil, fmt.Errorf("out_of_range: integer %s is outside the canonical int64 domain [-9223372036854775808, 9223372036854775807]; model larger values as strings (SPEC 2.3.2)", s)
				}
				return s, nil // not a parseable integer; fall through to bare string
			}
			return n, nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return s, nil // fall through to bare string
		}
		return f, nil
	}

	// 7. Bare string.
	return s, nil
}

// missingMarker represents an absent field in a tabular row.
type missingMarker struct{}

// attachmentMarker represents a nested value placeholder in a tabular row.
type attachmentMarker struct{}

// parseQuotedString parses a JSON-compatible quoted string.
func parseQuotedString(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' {
		return "", fmt.Errorf("unterminated_quote: missing opening quote")
	}

	var b strings.Builder
	i := 1
	for i < len(s) {
		if s[i] == '"' {
			// Closing quote.
			if i+1 != len(s) {
				return "", fmt.Errorf("trailing_characters: characters after closing quote")
			}
			return b.String(), nil
		}
		if s[i] == '\\' {
			if i+1 >= len(s) {
				return "", fmt.Errorf("unterminated_quote: escape at end of string")
			}
			i++
			switch s[i] {
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case '/':
				b.WriteByte('/')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if i+4 >= len(s) {
					return "", fmt.Errorf("invalid_escape: incomplete unicode escape")
				}
				hex := s[i+1 : i+5]
				code, err := strconv.ParseUint(hex, 16, 16)
				if err != nil {
					return "", fmt.Errorf("invalid_escape: invalid unicode escape \\u%s", hex)
				}
				r := rune(code)
				// Handle surrogate pairs.
				if r >= 0xD800 && r <= 0xDBFF {
					// High surrogate: must be followed by low surrogate.
					if i+10 >= len(s) || s[i+5] != '\\' || s[i+6] != 'u' {
						return "", fmt.Errorf("invalid_surrogate: isolated high surrogate")
					}
					hex2 := s[i+7 : i+11]
					code2, err := strconv.ParseUint(hex2, 16, 16)
					if err != nil {
						return "", fmt.Errorf("invalid_surrogate: invalid low surrogate \\u%s", hex2)
					}
					low := rune(code2)
					if low < 0xDC00 || low > 0xDFFF {
						return "", fmt.Errorf("invalid_surrogate: expected low surrogate, got \\u%s", hex2)
					}
					combined := 0x10000 + (r-0xD800)*0x400 + (low - 0xDC00)
					b.WriteRune(combined)
					i += 11
					continue
				}
				if r >= 0xDC00 && r <= 0xDFFF {
					return "", fmt.Errorf("invalid_surrogate: isolated low surrogate")
				}
				b.WriteRune(r)
				i += 5
				continue
			default:
				return "", fmt.Errorf("invalid_escape: unknown escape sequence \\%c", s[i])
			}
			i++
			continue
		}
		// Check for control characters.
		if s[i] < 0x20 {
			return "", fmt.Errorf("invalid_escape: unescaped control character U+%04X", s[i])
		}
		// Regular character (could be multi-byte UTF-8).
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			return "", fmt.Errorf("invalid_utf8: malformed UTF-8 byte sequence")
		}
		b.WriteRune(r)
		i += size
	}
	return "", fmt.Errorf("unterminated_quote: missing closing quote")
}

// splitRespectingQuotes splits a string on a delimiter, but not inside quoted strings.
func splitRespectingQuotes(s string, delim byte) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' && inQuote {
			current.WriteByte(c)
			escaped = true
			continue
		}
		if c == '"' {
			inQuote = !inQuote
			current.WriteByte(c)
			continue
		}
		if c == delim && !inQuote {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(c)
	}
	parts = append(parts, current.String())
	return parts
}

// splitFieldDecl splits a field declaration like {id,"display name","a,b",score}
// respecting quoted field names.
func splitFieldDecl(s string) ([]string, error) {
	// Strip braces.
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return nil, fmt.Errorf("invalid field declaration: %s", s)
	}
	inner := s[1 : len(s)-1]
	if inner == "" {
		return nil, nil
	}
	raw := splitRespectingQuotes(inner, ',')
	fields := make([]string, len(raw))
	for i, f := range raw {
		f = strings.TrimSpace(f)
		if len(f) >= 2 && f[0] == '"' && f[len(f)-1] == '"' {
			parsed, err := parseQuotedString(f)
			if err != nil {
				return nil, err
			}
			fields[i] = parsed
		} else {
			if !isBareKey(f) {
				return nil, fmt.Errorf("invalid field name: %s", f)
			}
			fields[i] = f
		}
	}
	// Check for duplicates.
	seen := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if _, ok := seen[f]; ok {
			return nil, fmt.Errorf("duplicate_field_name: %s", f)
		}
		seen[f] = struct{}{}
	}
	return fields, nil
}
