package eval

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

func Evaluate(expression string, config map[string]any) any {
	terms := regexp.MustCompile(`{{.*?}}|[^{}]+`).FindAllString(expression, -1)
	switch len(terms) {
	case 0:
		return ""
	case 1:
		return evaluateTerm(terms[0], config)
	default:
		result := ""
		for _, term := range terms {
			result += fmt.Sprintf("%v", evaluateTerm(term, config))
		}
		return result
	}
}

func evaluateTerm(term string, config map[string]any) any {
	if !strings.HasPrefix(term, "{{") || !strings.HasSuffix(term, "}}") {
		return term
	}

	path, functions, foundFunction := strings.Cut(term[2:len(term)-2], "|")
	value := dig(path, config)

	if !foundFunction {
		return value
	}

	for f := range strings.SplitSeq(functions, "|") {
		switch strings.TrimSpace(f) {
		case "volume":
			value = evaluate(value, volume)
		case "volume-target":
			value = evaluate(value, volumeTarget)
		case "into":
			value = into(value)
		case "first":
			value = first(value)
		case "last":
			value = last(value)
		default:
			if strings.HasPrefix(f, "or:") {
				_, rest, _ := strings.Cut(f, ":")
				value = or(value, rest)
			} else if strings.HasPrefix(f, "mount_as:") {
				_, rest, _ := strings.Cut(f, ":")
				value = mountAs(value, rest)
			}
		}
	}

	return value
}

func evaluate(value any, fn func(v any) string) any {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Slice {
		list := make([]string, v.Len())
		for i := range list {
			list[i] = fn(v.Index(i).Interface())
		}
		return list
	}

	return fn(v)
}

func EvaluateList(expressions []string, arguments map[string]any) []string {
	var replaced []string

	for _, expression := range expressions {
		value := Evaluate(expression, arguments)

		v := reflect.ValueOf(value)
		if v.Kind() == reflect.Slice {
			for i := range v.Len() {
				replaced = append(replaced, fmt.Sprintf("%v", v.Index(i).Interface()))
			}
		} else {
			replaced = append(replaced, v.String())
		}
	}

	return replaced
}

func dig(key string, config map[string]any) any {
	key = strings.TrimSpace(key)

	top, rest, found := strings.Cut(key, ".")
	if !found {
		value := config[key]
		if value == nil {
			return ""
		}
		return config[key]
	}

	top = strings.TrimSpace(top)
	childConfig, ok := config[top].(map[string]any)
	if !ok {
		return ""
	}

	return dig(rest, childConfig)
}
