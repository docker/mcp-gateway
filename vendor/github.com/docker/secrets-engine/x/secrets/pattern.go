// Copyright 2025-2026 Docker, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package secrets

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidPattern = errors.New("invalid pattern")

// validPattern checks if a pattern is valid without using regexp or unicode.
// Rules:
// - Components separated by '/'
// - Each component is non-empty
// - Only characters A-Z, a-z, 0-9, '.', '-', '_' or '*'
// - No leading, trailing, or double slashes
// - Asterisks rules:
//   - '*' cannot be mixed with other characters in the same component
//   - there can be no more than two '*' per component
func validPattern(s string) bool {
	if len(s) == 0 {
		return false
	}

	componentLen := 0
	wildcardLen := 0

	for _, r := range s {
		switch {
		case r == '/':
			if !isValidComponentMatcher(componentLen, wildcardLen) {
				return false
			}
			componentLen = 0
			wildcardLen = 0
		case isValidPatternRune(r):
			componentLen++
			if r == '*' {
				wildcardLen++
			}
		default:
			return false
		}
	}
	// Final component
	return isValidComponentMatcher(componentLen, wildcardLen)
}

func isValidComponentMatcher(componentLen, wildcardLen int) bool {
	if wildcardLen > 2 {
		// No more than two wildcards per component
		return false
	}
	if wildcardLen > 0 && wildcardLen != componentLen {
		// Wildcard can't be mixed with other characters in the same component
		return false
	}
	// Component must not be empty
	return componentLen > 0
}

func isValidPatternRune(c rune) bool {
	return isValidRune(c) || c == '*'
}

// Pattern can be used to match secret identifiers.
// Valid patterns must follow the same validation rules as secret identifiers, with the exception
// that '*' can be used to match a single component, and '**' can be used to match zero or more components.
type Pattern interface {
	// Match the [Pattern] against an [ID]
	Match(id ID) bool
	// Includes returns true if all matches of Pattern [other] are also matches of the current pattern.
	Includes(other Pattern) bool
	// String formats the [Pattern] as a string
	String() string

	ExpandID(other ID) (ID, error)
	ExpandPattern(other Pattern) (Pattern, error)
}

type pattern string

func (p pattern) Match(id ID) bool {
	pathParts := split(id.String())
	patternParts := split(string(p))

	return match(patternParts, pathParts)
}

func (p pattern) Includes(other Pattern) bool {
	otherParts := split(other.String())
	patternParts := split(string(p))

	return match(patternParts, otherParts)
}

func (p pattern) String() string {
	return string(p)
}

// ParsePattern parses a string into a [Pattern]
// Rules:
// - Components separated by '/'
// - Each component is non-empty
// - Only characters A-Z, a-z, 0-9, '.', '-', '_' or '*'
// - No leading, trailing, or double slashes
// - Asterisks rules:
//   - '*' cannot be mixed with other characters in the same component
//   - there can be no more than two '*' per component
func ParsePattern(s string) (Pattern, error) {
	if !validPattern(s) {
		return nil, ErrInvalidPattern
	}
	return pattern(s), nil
}

// MustParsePattern parses a string into a [Pattern] like with [ParsePattern],
// however, it panics when a validation error occurs.
func MustParsePattern(s string) Pattern {
	if !validPattern(s) {
		panic(ErrInvalidPattern)
	}
	return pattern(s)
}

func (p pattern) ExpandID(other ID) (ID, error) {
	val, err := replace1(string(p), other.String())
	if err != nil {
		return nil, err
	}
	return id(val), err
}

func (p pattern) ExpandPattern(other Pattern) (Pattern, error) {
	val, err := replace1(string(p), other.String())
	if err != nil {
		return nil, err
	}
	return pattern(val), err
}

// Filter returns a reduced [Pattern] that is subset equal to [filter].
// Returns false if there's no overlap between [filter] and [other].
// Examples:
// - Filter(MustParsePattern("bar/**"), MustParsePattern("**")) => returns "bar/**"
// - Filter(MustParsePattern("**"), MustParsePattern("**")) => returns "**"
// - Filter(MustParsePattern("bar/**"), MustParsePattern("bar")) => returns "bar"
// - Filter(MustParsePattern("bar/**"), MustParsePattern("foo/**")) => returns false
func Filter(filter, other Pattern) (Pattern, bool) {
	if filter.Includes(other) {
		return other, true
	}
	if other.Includes(filter) {
		return filter, true
	}
	return nil, false
}

func replace1(original, other string) (string, error) {
	components := split(original)
	var candidates []int
	for idx, val := range components {
		if val == "**" {
			candidates = append(candidates, idx)
		}
	}
	if len(candidates) != 1 {
		return "", fmt.Errorf("expand only supports one expansion glob, pattern %s has %d", original, len(candidates))
	}
	components[candidates[0]] = other
	return strings.Join(components, "/"), nil
}
