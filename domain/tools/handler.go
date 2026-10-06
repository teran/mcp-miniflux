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

// arrayProp returns a JSON schema for an array whose items match the given
// object item schema.
func arrayProp(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

// categoryRefProps returns the properties of the CategoryRef object embedded in
// a Feed.
func categoryRefProps() map[string]any {
	return map[string]any{
		"id":    integerProp(),
		"title": stringProp(),
	}
}

// feedProps returns the properties of a Feed object (M07/S09). Username and
// Password are redacted (S02) but still present as empty strings in the emitted
// JSON (no omitempty), so the schema must declare them as strings.
func feedProps() map[string]any {
	return map[string]any{
		"id":          integerProp(),
		"user_id":     integerProp(),
		"feed_url":    stringProp(),
		"site_url":    stringProp(),
		"title":       stringProp(),
		"category":    *objectSchema(categoryRefProps(), nil),
		"status":      stringProp(),
		"error_count": integerProp(),
		"username":    stringProp(),
		"password":    stringProp(),
	}
}

// categoryProps returns the properties of a Category object.
func categoryProps() map[string]any {
	return map[string]any{
		"id":          integerProp(),
		"title":       stringProp(),
		"feed_count":  integerProp(),
		"entry_count": integerProp(),
	}
}

// entryProps returns the properties of an Entry object. Time fields are
// marshalled as RFC3339 strings.
func entryProps() map[string]any {
	return map[string]any{
		"id":           integerProp(),
		"user_id":      integerProp(),
		"feed_id":      integerProp(),
		"status":       stringProp(),
		"starred":      booleanProp(),
		"title":        stringProp(),
		"url":          stringProp(),
		"comments_url": stringProp(),
		"published_at": stringProp(),
		"created_at":   stringProp(),
		"content":      stringProp(),
	}
}

// countersProps returns the properties of a Counters object. `feeds` is a
// map[string]CounterTotals keyed by feed id (each value an object of
// {read,unread}), declared as a permissive object.
func countersProps() map[string]any {
	return map[string]any{
		"feeds":  map[string]any{"type": "object"},
		"totals": *objectSchema(map[string]any{"unread": integerProp(), "read": integerProp()}, nil),
	}
}

// meProps returns the properties of a Me object.
func meProps() map[string]any {
	return map[string]any{
		"id":       integerProp(),
		"username": stringProp(),
		"is_admin": booleanProp(),
		"theme":    stringProp(),
	}
}

// feedEntriesProps returns the properties of a FeedEntries (paginated entries)
// object.
func feedEntriesProps() map[string]any {
	return map[string]any{
		"total":   integerProp(),
		"entries": arrayProp(*objectSchema(entryProps(), nil)),
	}
}

// okProps returns the properties of a {"ok": bool} acknowledgement object.
func okProps() map[string]any {
	return map[string]any{"ok": booleanProp()}
}

// opmlProps returns the properties of an {"opml": string} object.
func opmlProps() map[string]any {
	return map[string]any{"opml": stringProp()}
}

// listSchema returns a strict output schema wrapping items of the given object
// item schema under the given list key.
func listSchema(key string, items map[string]any) *map[string]any {
	return objectSchema(map[string]any{key: arrayProp(items)}, nil)
}

// feedSchema returns a strict output schema for a Feed object.
func feedSchema() *map[string]any { return objectSchema(feedProps(), nil) }

// categorySchema returns a strict output schema for a Category object.
func categorySchema() *map[string]any { return objectSchema(categoryProps(), nil) }

// entrySchema returns a strict output schema for an Entry object.
func entrySchema() *map[string]any { return objectSchema(entryProps(), nil) }

// feedEntriesSchema returns a strict output schema for a FeedEntries object.
func feedEntriesSchema() *map[string]any { return objectSchema(feedEntriesProps(), nil) }

// okSchema returns a strict output schema for a {"ok": bool} object.
func okSchema() *map[string]any { return objectSchema(okProps(), nil) }
