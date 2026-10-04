package miniflux

import (
	"reflect"
	"strings"
)

// Redact returns a deep copy of v in which every struct field annotated
// `secret:"true"` is zeroed (S02/L05). It recurses into nested structs,
// pointers, slices and maps and never mutates the input. Non-struct values
// (string, int, nil, ...) are returned unchanged; nil pointers/slices/maps
// are returned as an untyped nil.
func Redact(v any) any {
	if v == nil {
		return nil
	}
	val := redactValue(reflect.ValueOf(v))
	if isNilValue(val) {
		return nil
	}
	return val.Interface()
}

// isNilValue reports whether v is a nil-able kind holding a nil value (so the
// caller can return an untyped nil rather than a typed-nil interface).
func isNilValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}

// redactValue returns a new reflect.Value of the same type as v with all
// `secret:"true"` fields zeroed. It is the recursive workhorse behind Redact.
func redactValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}

	switch v.Kind() {
	case reflect.Struct:
		return redactStruct(v)
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(redactValue(v.Elem()))
		return out
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(redactValue(v.Index(i)))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), redactValue(iter.Value()))
		}
		return out
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		return redactValue(v.Elem())
	default:
		return v
	}
}

// redactStruct rebuilds a struct, zeroing `secret:"true"` fields and
// recursing into the rest.
func redactStruct(v reflect.Value) reflect.Value {
	out := reflect.New(v.Type()).Elem()
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		if field.Tag.Get("secret") == "true" {
			out.Field(i).Set(reflect.Zero(field.Type))
		} else {
			out.Field(i).Set(redactValue(v.Field(i)))
		}
	}
	return out
}

// secretKeySet is the set of lowercased map keys that RedactMap removes (L05).
// It is deliberately name-based because tool-output and access-log `args` are
// arbitrary map[string]any with no structural schema.
var secretKeySet = map[string]struct{}{
	"password":  {},
	"token":     {},
	"apikey":    {},
	"api_token": {},
}

// RedactMap returns a deep copy of m in which every key whose lowercased name
// is in the secret-name set is removed, recursing into nested map[string]any
// and []any values. The input map is never modified.
func RedactMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}

	out := make(map[string]any, len(m))
	for k, v := range m {
		if _, ok := secretKeySet[strings.ToLower(k)]; ok {
			continue
		}
		out[k] = redactMapValue(v)
	}
	return out
}

// redactMapValue recurses into nested map[string]any and []any values,
// forwarding anything else unchanged.
func redactMapValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return RedactMap(t)
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = redactMapValue(item)
		}
		return out
	default:
		return v
	}
}
