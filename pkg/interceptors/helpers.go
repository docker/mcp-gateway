package interceptors

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func argumentsToString(args any) string {
	buf, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("%v", args)
	}

	return string(buf)
}

func argumentsSummary(args any) string {
	if args == nil {
		return "none"
	}

	switch v := args.(type) {
	case json.RawMessage:
		return rawJSONSummary(v)
	case []byte:
		return rawJSONSummary(v)
	}

	value := reflect.ValueOf(args)
	if !value.IsValid() {
		return "none"
	}

	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return "none"
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Map:
		return fmt.Sprintf("object with %d field(s)", value.Len())
	case reflect.Slice, reflect.Array:
		return fmt.Sprintf("array with %d item(s)", value.Len())
	case reflect.Struct:
		return fmt.Sprintf("object with %d field(s)", value.NumField())
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return "number"
	default:
		return value.Kind().String()
	}
}

func rawJSONSummary(raw []byte) string {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "raw JSON"
	}
	return argumentsSummary(decoded)
}
