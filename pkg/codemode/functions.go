package codemode

import (
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func toolToJsDoc(tool *mcp.Tool) string {
	var doc strings.Builder

	doc.WriteString("===== " + tool.Name + " =====\n\n")
	doc.WriteString(strings.TrimSpace(tool.Description))
	doc.WriteString("\n\n")

	// Extract schema properties and required fields from InputSchema
	var properties map[string]any
	var required []string

	if tool.InputSchema != nil {
		if schemaMap, ok := tool.InputSchema.(map[string]any); ok {
			if props, ok := schemaMap["properties"].(map[string]any); ok {
				properties = props
			}
			if req, ok := schemaMap["required"].([]any); ok {
				for _, r := range req {
					if reqStr, ok := r.(string); ok {
						required = append(required, reqStr)
					}
				}
			} else if req, ok := schemaMap["required"].([]string); ok {
				required = req
			}
		}
	}

	if len(properties) == 0 {
		doc.WriteString(fmt.Sprintf("%s(): string\n", tool.Name))
	} else {
		doc.WriteString(fmt.Sprintf("%s(args: ArgsObject): string\n", tool.Name))
		doc.WriteString("\nwhere type ArgsObject = {\n")
		for paramName, param := range properties {
			pType := "Object"

			var (
				pDesc string
				pEnum string
			)
			if paramMap, ok := param.(map[string]any); ok {
				if t, ok := paramMap["type"].(string); ok {
					pType = t
				}
				if d, ok := paramMap["description"].(string); ok {
					pDesc = d
				}
				if values, ok := paramMap["enum"].([]any); ok {
					for _, v := range values {
						if pEnum != "" {
							pEnum += " | "
						}
						if pType == "string" {
							pEnum += fmt.Sprintf("'%v'", v)
						} else {
							pEnum += fmt.Sprintf("%v", v)
						}
					}
				}
			}

			if !slices.Contains(required, paramName) {
				paramName += "?"
			}

			if pEnum != "" {
				doc.WriteString(fmt.Sprintf("  %s: %s // %s\n", paramName, pEnum, strings.TrimSpace(pDesc)))
			} else {
				doc.WriteString(fmt.Sprintf("  %s: %s // %s\n", paramName, pType, strings.TrimSpace(pDesc)))
			}
		}
		doc.WriteString("};\n")
	}

	return doc.String()
}
