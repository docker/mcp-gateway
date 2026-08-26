package gcf

import (
	"fmt"
	"strconv"
	"strings"
)

func parseHeaderProfile(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) < 2 {
		return "", fmt.Errorf("missing_profile: header has no fields")
	}
	seen := make(map[string]struct{})
	var profile string
	for _, p := range parts[1:] {
		eqIdx := strings.Index(p, "=")
		if eqIdx < 0 {
			return "", fmt.Errorf("malformed_header_field: %s", p)
		}
		key := p[:eqIdx]
		if _, ok := seen[key]; ok {
			return "", fmt.Errorf("duplicate_header_field: %s", key)
		}
		seen[key] = struct{}{}
		if key == "profile" {
			profile = p[eqIdx+1:]
		}
	}
	if profile == "" {
		return "", fmt.Errorf("missing_profile: no profile= field")
	}
	return profile, nil
}

func findBracketStart(s string) int {
	// Find " [" (the named-array count bracket) that is OUTSIDE any quoted name,
	// so a quoted section/key name containing " [" (e.g. `## "a [1] b"`) is not
	// misread as a named-array header.
	inQuote := false
	escaped := false
	for i := 0; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		c := s[i]
		if c == '\\' && inQuote {
			escaped = true
			continue
		}
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && c == ' ' && i+1 < len(s) && s[i+1] == '[' {
			return i
		}
	}
	return -1
}

// findInlineArrayBracket finds "[" in content for inline arrays like name[N]:
// findClosingBrace finds the index of the closing } in a field declaration,
// respecting quoted field names that may contain }.
func findClosingBrace(s string) int {
	inQuote := false
	escaped := false
	for i := 0; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		if s[i] == '\\' && inQuote {
			escaped = true
			continue
		}
		if s[i] == '"' {
			inQuote = !inQuote
			continue
		}
		if s[i] == '}' && !inQuote {
			return i
		}
	}
	return -1
}

// arrayBracketStart returns the index of the "[" that opens a named-array marker
// (name[N]:), scanning past a quoted key so a "[" inside the key name is not
// mistaken for the array bracket (SPEC 4.2). Bare keys cannot contain "[".
func arrayBracketStart(content string) int {
	if len(content) > 0 && content[0] == '"' {
		escaped := false
		for i := 1; i < len(content); i++ {
			if escaped {
				escaped = false
				continue
			}
			if content[i] == '\\' {
				escaped = true
				continue
			}
			if content[i] == '"' {
				if i+1 < len(content) && content[i+1] == '[' {
					return i + 1
				}
				return -1
			}
		}
		return -1
	}
	return strings.Index(content, "[")
}

// Must not start with "@" or "##".
func findInlineArrayBracket(content string) int {
	if strings.HasPrefix(content, "@") || strings.HasPrefix(content, "##") {
		return -1
	}
	// Find [ that's followed by digits/? and ]:
	idx := arrayBracketStart(content)
	if idx <= 0 {
		return -1
	}
	rest := content[idx:]
	closeIdx := strings.Index(rest, "]")
	if closeIdx < 0 {
		return -1
	}
	afterClose := rest[closeIdx+1:]
	if strings.HasPrefix(afterClose, ": ") || afterClose == ":" {
		return idx
	}
	return -1
}

// parseKeyFromHeaderContent parses a key from header content, handling quoting.
func parseKeyFromHeaderContent(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' {
		return parseQuotedString(s)
	}
	return s, nil
}

// findKeyValueSplit finds the = that separates key from value, handling quoted keys.
func findKeyValueSplit(s string) int {
	if len(s) == 0 {
		return -1
	}
	if s[0] == '"' {
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == '"' {
				if i+1 < len(s) && s[i+1] == '=' {
					return i + 1
				}
				return -1
			}
		}
		return -1
	}
	eqIdx := strings.Index(s, "=")
	if eqIdx < 0 {
		return -1
	}
	bracketIdx := strings.Index(s, "[")
	if bracketIdx >= 0 && bracketIdx < eqIdx {
		return -1
	}
	return eqIdx
}

func parseCount(s string) (int, error) {
	if s == "0" {
		return 0, nil
	}
	if len(s) == 0 || s[0] == '0' {
		return 0, fmt.Errorf("invalid_count: %s", s)
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid_count: %s", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func payloadToMap(p *Payload) map[string]any {
	syms := make([]any, len(p.Symbols))
	for i, s := range p.Symbols {
		syms[i] = map[string]any{
			"qualifiedName": s.QualifiedName,
			"kind":          s.Kind,
			"score":         s.Score,
			"provenance":    s.Provenance,
			"distance":      s.Distance,
		}
	}
	edges := make([]any, len(p.Edges))
	for i, e := range p.Edges {
		m := map[string]any{
			"source":   e.Source,
			"target":   e.Target,
			"edgeType": e.EdgeType,
			"status":   e.Status,
		}
		edges[i] = m
	}
	return map[string]any{
		"tool":        p.Tool,
		"tokenBudget": p.TokenBudget,
		"tokensUsed":  p.TokensUsed,
		"packRoot":    p.PackRoot,
		"symbols":     syms,
		"edges":       edges,
	}
}

// validateSummaryCounts validates ##! summary counts against deferred sections.
func validateSummaryCounts(summaryLine string, deferredCount int, contentLines []string) error {
	// Parse counts from "##! summary counts=N,M,..."
	parts := strings.Fields(summaryLine)
	var countsStr string
	for _, p := range parts {
		if strings.HasPrefix(p, "counts=") {
			countsStr = p[7:]
			break
		}
	}
	if countsStr == "" {
		return nil // no counts field
	}

	countVals := strings.Split(countsStr, ",")
	if len(countVals) != deferredCount {
		return fmt.Errorf("count_mismatch: summary has %d count entries but %d deferred sections", len(countVals), deferredCount)
	}

	// Count actual items per deferred section.
	var actualCounts []int
	inDeferred := false
	currentCount := 0
	for _, l := range contentLines {
		trimmed := strings.TrimLeft(l, " ")
		if strings.HasPrefix(trimmed, "## ") && (strings.Contains(trimmed, "[?]") || strings.Contains(trimmed, "[?:]")) {
			if inDeferred {
				actualCounts = append(actualCounts, currentCount)
			}
			inDeferred = true
			currentCount = 0
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			if inDeferred {
				actualCounts = append(actualCounts, currentCount)
				inDeferred = false
			}
			continue
		}
		if inDeferred {
			// Count data lines (non-indented relative to section).
			if !strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(trimmed, ".") {
				currentCount++
			}
		}
	}
	if inDeferred {
		actualCounts = append(actualCounts, currentCount)
	}

	for i, cv := range countVals {
		declared, err := strconv.Atoi(cv)
		if err != nil {
			return fmt.Errorf("count_mismatch: invalid count value %q", cv)
		}
		if i < len(actualCounts) && declared != actualCounts[i] {
			return fmt.Errorf("count_mismatch: section %d declared %d in summary, actual %d", i, declared, actualCounts[i])
		}
	}

	return nil
}
