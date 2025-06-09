package eval

import (
	"fmt"
	"reflect"
)

func last(value any) string {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Slice {
		if v.Len() == 0 {
			return ""
		}

		return fmt.Sprintf("%v", v.Index(v.Len()-1).Interface())
	}

	return fmt.Sprintf("%v", value)
}
