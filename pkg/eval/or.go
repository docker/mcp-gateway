package eval

import (
	"fmt"
	"reflect"
	"strings"
)

func or(value any, defaultValue string) any {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Slice {
		if v.Len() != 0 {
			return value
		}
		return orDefault(defaultValue)
	}

	if value == nil || value == "" {
		return orDefault(defaultValue)
	}

	return fmt.Sprintf("%v", value)
}

// orDefault returns the default of an `or:` function. A default written as
// `[a,b]` is a list, and `[]` is an empty list.
func orDefault(defaultValue string) any {
	if strings.HasPrefix(defaultValue, "[") && strings.HasSuffix(defaultValue, "]") {
		commaSeparatedValues := defaultValue[1 : len(defaultValue)-1]
		if len(commaSeparatedValues) == 0 {
			return []string{}
		}
		return strings.Split(commaSeparatedValues, ",")
	}
	return defaultValue
}
