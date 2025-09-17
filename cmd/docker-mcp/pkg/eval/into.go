package eval

import (
	"fmt"
	"reflect"
)

func into(value any) []string {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Slice {
		list := make([]string, v.Len())
		for i := range list {
			list[i] = fmt.Sprintf("%v", v.Index(i).Interface())
		}
		return list
	}

	return []string{fmt.Sprintf("%v", value)}
}
