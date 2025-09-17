package eval

import (
	"fmt"
	"reflect"
	"strings"
)

func mountAs(value any, asPath string) string {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Slice {
		if v.Len() == 0 {
			return ""
		}

		value = v.Index(0).Interface()
	}

	if value == nil || value == "" {
		return ""
	}

	return fmt.Sprintf("%v:%s", value, strings.TrimSpace(asPath))
}
