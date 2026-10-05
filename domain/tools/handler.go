// Package tools declares the MCP tool surface (SPEC §4, §6.2; S02/S03/S08/S09/
// S12). Each tool is a struct implementing the Handler interface; the zero value
// exposes all definition methods (Name/InputSchema/OutputSchema/Annotations/
// Instructions) without any injected dependency. Execution lives in the
// application layer (application/handlers); this package is definition-only.
package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Handler is the common interface every tool implements (SPEC §6.2). It exposes
// only the definition surface (name, schemas, annotations, instructions);
// execution is wired in the application layer via the dmf.Client port.
type Handler interface {
	Name() string
	InputSchema() *map[string]any
	OutputSchema() *map[string]any
	Annotations() mcp.ToolAnnotations
	Instructions() string
}

// ptrBool returns a pointer to b.
func ptrBool(b bool) *bool { return &b }

// objectSchema builds a strict JSON object input schema with
// additionalProperties:false (S08).
func objectSchema(properties map[string]any, required []string) *map[string]any {
	s := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
	}
	if len(properties) > 0 {
		s["properties"] = properties
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return &s
}

// integerProp returns a JSON schema for an integer property.
func integerProp() map[string]any { return map[string]any{"type": "integer"} }

// stringProp returns a JSON schema for a string property.
func stringProp() map[string]any { return map[string]any{"type": "string"} }

// booleanProp returns a JSON schema for a boolean property.
func booleanProp() map[string]any { return map[string]any{"type": "boolean"} }

// stringEnumProp returns a JSON schema for a string property constrained to
// the given enum values.
func stringEnumProp(values ...string) map[string]any {
	enum := make([]any, 0, len(values))
	for _, v := range values {
		enum = append(enum, v)
	}
	return map[string]any{"type": "string", "enum": enum}
}

// integerArrayProp returns a JSON schema for an integer array property, capped
// at maxLen items.
func integerArrayProp(maxLen int) map[string]any {
	return map[string]any{
		"type":     "array",
		"items":    map[string]any{"type": "integer"},
		"maxItems": maxLen,
	}
}

// outputSchema builds a permissive output schema (S09 requires every tool to
// declare one). Outputs are refined by the application layer; here we assert an
// object so the contract is complete and non-nil.
func outputSchema() *map[string]any {
	return objectSchema(nil, nil)
}
