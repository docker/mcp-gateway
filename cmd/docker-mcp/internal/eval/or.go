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

		if strings.HasPrefix(defaultValue, "[") && strings.HasSuffix(defaultValue, "]") {
			commaSeparatedValues := defaultValue[1 : len(defaultValue)-1]
			if len(commaSeparatedValues) == 0 {
				return []string{}
			}
			return strings.Split(commaSeparatedValues, ",")
		}
		return fmt.Sprintf("%v", defaultValue)
	}

	if value == nil || value == "" {
		return defaultValue
	}

	return fmt.Sprintf("%v", value)
}
