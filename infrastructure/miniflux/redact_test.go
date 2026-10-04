package miniflux

// CONTRACT — redaction helper (SPEC §7 S02, §9 L05). The developer must define
// the following symbols in package infrastructure/miniflux. This test file is
// written FIRST (TDD, red state) and will not compile until they exist.
//
//	// Redact returns a deep copy of v with every field annotated `secret:"true"`
//	// zeroed. It recurses into nested structs, pointers, slices and maps, and
//	// MUST NOT mutate the input. Non-struct values (string, int, nil, ...) are
//	// returned unchanged.
//	func Redact(v any) any
//
//	// RedactMap returns a deep copy of m in which every key whose lowercased
//	// name is in the secret-name set {"password","token","apikey","api_token"}
//	// is removed (recursively into nested map[string]any and []any values).
//	// The input map is NOT modified.
//	func RedactMap(m map[string]any) map[string]any
//
// Decision notes:
//   - Redact is annotation-based (S02): only fields carrying `secret:"true"`
//     are stripped; everything else is preserved verbatim.
//   - RedactMap is name-based (pragmatic, documented): tool-output and
//     access-log `args` are arbitrary `map[string]any`, so a structural schema
//     is not always available. A key is redacted when its lowercased name is in
//     the set above; matching is case-insensitive.

import (
	"reflect"
	"testing"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// local test types to lock recursion into nested structs (S02).

type redactNestedSecret struct {
	Name  string `json:"name"`
	Token string `json:"token" secret:"true"`
}

type redactOuter struct {
	Label  string             `json:"label"`
	Inner  redactNestedSecret `json:"inner"`
	Secret string             `json:"secret" secret:"true"`
}

func TestRedactTopLevelStruct(t *testing.T) {
	f := dmf.Feed{ID: 42, Title: "Example", Username: "svc", Password: "topsecret"}

	got, ok := Redact(f).(dmf.Feed)
	if !ok {
		t.Fatalf("Redact(feed) = %T, want dmf.Feed", Redact(f))
	}
	if got.Password != "" || got.Username != "" {
		t.Errorf("secrets not zeroed: username=%q password=%q", got.Username, got.Password)
	}
	if got.ID != 42 || got.Title != "Example" {
		t.Errorf("non-secret fields not preserved: %+v", got)
	}
	// Original must not be mutated.
	if f.Password != "topsecret" || f.Username != "svc" {
		t.Errorf("Redact mutated the original: %+v", f)
	}
}

func TestRedactNestedStruct(t *testing.T) {
	o := redactOuter{Label: "keep", Inner: redactNestedSecret{Name: "n", Token: "inner-secret"}, Secret: "outer-secret"}

	got, ok := Redact(o).(redactOuter)
	if !ok {
		t.Fatalf("Redact(outer) = %T, want redactOuter", Redact(o))
	}
	if got.Secret != "" {
		t.Errorf("outer secret not zeroed: %q", got.Secret)
	}
	if got.Inner.Token != "" {
		t.Errorf("nested secret not zeroed: %q", got.Inner.Token)
	}
	if got.Label != "keep" || got.Inner.Name != "n" {
		t.Errorf("non-secret fields not preserved: %+v", got)
	}
	if o.Secret != "outer-secret" || o.Inner.Token != "inner-secret" {
		t.Errorf("Redact mutated the original: %+v", o)
	}
}

func TestRedactSliceOfStructs(t *testing.T) {
	feeds := []dmf.Feed{
		{ID: 1, Title: "A", Username: "u1", Password: "p1"},
		{ID: 2, Title: "B", Password: "p2"},
	}

	got, ok := Redact(feeds).([]dmf.Feed)
	if !ok {
		t.Fatalf("Redact(slice) = %T, want []dmf.Feed", Redact(feeds))
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for i := range got {
		if got[i].Password != "" || got[i].Username != "" {
			t.Errorf("feed[%d] secrets not zeroed: %+v", i, got[i])
		}
	}
	if got[0].Title != "A" || got[0].ID != 1 {
		t.Errorf("feed[0] non-secret not preserved: %+v", got[0])
	}
	if feeds[0].Password != "p1" || feeds[1].Password != "p2" {
		t.Errorf("Redact mutated the original slice: %+v", feeds)
	}
}

func TestRedactMapValueContainingSecrets(t *testing.T) {
	m := map[string]dmf.Feed{
		"a": {ID: 1, Title: "A", Password: "secret-a"},
		"b": {ID: 2, Title: "B", Username: "u", Password: "secret-b"},
	}

	got, ok := Redact(m).(map[string]dmf.Feed)
	if !ok {
		t.Fatalf("Redact(map) = %T, want map[string]dmf.Feed", Redact(m))
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for k, v := range got {
		if v.Password != "" || v.Username != "" {
			t.Errorf("map[%q] secrets not zeroed: %+v", k, v)
		}
	}
	if got["a"].Title != "A" {
		t.Errorf("map[\"a\"] non-secret not preserved: %+v", got["a"])
	}
	if m["a"].Password != "secret-a" || m["b"].Password != "secret-b" {
		t.Errorf("Redact mutated the original map: %+v", m)
	}
}

func TestRedactPointerToStruct(t *testing.T) {
	f := &dmf.Feed{ID: 9, Title: "P", Username: "pu", Password: "pp"}

	got, ok := Redact(f).(*dmf.Feed)
	if !ok {
		t.Fatalf("Redact(ptr) = %T, want *dmf.Feed", Redact(f))
	}
	if got.Password != "" || got.Username != "" {
		t.Errorf("pointer secrets not zeroed: %+v", got)
	}
	if got.ID != 9 || got.Title != "P" {
		t.Errorf("pointer non-secret not preserved: %+v", got)
	}
	if f.Password != "pp" || f.Username != "pu" {
		t.Errorf("Redact mutated the pointed-to value: %+v", f)
	}
}

func TestRedactNilPointer(t *testing.T) {
	var p *dmf.Feed
	if got := Redact(p); got != nil {
		t.Errorf("Redact(nil ptr) = %#v, want nil", got)
	}
}

func TestRedactNonStructPassThrough(t *testing.T) {
	if got := Redact("hello"); got != "hello" {
		t.Errorf("Redact(string) = %#v, want \"hello\"", got)
	}
	if got, ok := Redact(42).(int); !ok || got != 42 {
		t.Errorf("Redact(int) = %#v, want 42", Redact(42))
	}
	if got := Redact(nil); got != nil {
		t.Errorf("Redact(nil) = %#v, want nil", got)
	}
	if got, ok := Redact(3.14).(float64); !ok || got != 3.14 {
		t.Errorf("Redact(float64) = %#v, want 3.14", Redact(3.14))
	}
}

func TestRedactRecursesIntoNestedSliceOfStructs(t *testing.T) {
	o := redactOuter{
		Label:  "keep",
		Inner:  redactNestedSecret{Name: "n", Token: "t1"},
		Secret: "s1",
	}
	// Wrap the struct in a slice to prove recursion depth is not capped at one
	// level.
	got, ok := Redact([]redactOuter{o}).([]redactOuter)
	if !ok || len(got) != 1 {
		t.Fatalf("Redact([]redactOuter) = %#v", Redact([]redactOuter{o}))
	}
	if got[0].Secret != "" || got[0].Inner.Token != "" {
		t.Errorf("nested-in-slice secrets not zeroed: %+v", got[0])
	}
}

func TestRedactMap(t *testing.T) {
	m := map[string]any{
		"password": "s3cret",
		"token":    "tok-val",
		"title":    "ok",
		"nested": map[string]any{
			"api_token": "def",
			"keep":      1,
		},
		"list": []any{
			map[string]any{"apikey": "abc", "name": "x"},
		},
	}

	out := RedactMap(m)

	// Original unmodified.
	if m["password"] != "s3cret" || m["token"] != "tok-val" {
		t.Fatalf("RedactMap mutated the original: %v", m)
	}

	for _, key := range []string{"password", "token"} {
		if _, ok := out[key]; ok {
			t.Errorf("secret key %q still present: %v", key, out)
		}
	}
	if out["title"] != "ok" {
		t.Errorf("non-secret key lost: %v", out)
	}

	nested, ok := out["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested = %T, want map[string]any", out["nested"])
	}
	if _, ok := nested["api_token"]; ok {
		t.Errorf("nested api_token still present: %v", nested)
	}
	if nested["keep"] != 1 {
		t.Errorf("nested non-secret key lost: %v", nested)
	}

	list, ok := out["list"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("list = %#v", out["list"])
	}
	item, ok := list[0].(map[string]any)
	if !ok {
		t.Fatalf("list[0] = %T, want map[string]any", list[0])
	}
	if _, ok := item["apikey"]; ok {
		t.Errorf("list item apikey still present: %v", item)
	}
	if item["name"] != "x" {
		t.Errorf("list item non-secret key lost: %v", item)
	}
}

func TestRedactMapCaseInsensitive(t *testing.T) {
	m := map[string]any{
		"PASSWORD":  "up",
		"API_TOKEN": "up2",
		"Token":     "up3",
		"Title":     "keep",
	}
	out := RedactMap(m)
	for _, key := range []string{"PASSWORD", "API_TOKEN", "Token"} {
		if _, ok := out[key]; ok {
			t.Errorf("case-insensitive secret key %q still present: %v", key, out)
		}
	}
	if out["Title"] != "keep" {
		t.Errorf("non-secret key lost: %v", out)
	}
}

func TestRedactMapNil(t *testing.T) {
	if got := RedactMap(nil); got != nil {
		t.Errorf("RedactMap(nil) = %#v, want nil", got)
	}
}

// TestRedactMapDoesNotRemoveUnrelatedKeys guards against over-aggressive
// redaction (only the documented secret names are removed).
func TestRedactMapDoesNotRemoveUnrelatedKeys(t *testing.T) {
	m := map[string]any{"password": "x", "category": "tech", "title": "t"}
	out := RedactMap(m)
	for _, key := range []string{"category", "title"} {
		if _, ok := out[key]; !ok {
			t.Errorf("non-secret key %q removed: %v", key, out)
		}
	}
}

func TestRedactMapReturnsCopyNotAlias(t *testing.T) {
	m := map[string]any{"password": "s", "a": 1}
	out := RedactMap(m)
	// Mutating the output must not affect the input.
	out["a"] = 999
	if m["a"] != 1 {
		t.Errorf("output aliases input map: m=%v out=%v", m, out)
	}
}

// Redact on a type with no secret fields must be an identity copy (no panic,
// no data loss), and must not alias shared slices/maps.
func TestRedactIdentityCopyForPlainStruct(t *testing.T) {
	type plain struct {
		A int
		B []string
	}
	p := plain{A: 5, B: []string{"x", "y"}}
	got, ok := Redact(p).(plain)
	if !ok {
		t.Fatalf("Redact(plain) = %T", Redact(p))
	}
	if !reflect.DeepEqual(got, p) {
		t.Errorf("Redact(plain) not equal: got %+v want %+v", got, p)
	}
	// Slices must not alias the input.
	got.B[0] = "mutated"
	if p.B[0] != "x" {
		t.Errorf("Redact(plain) aliased slice: %v", p.B)
	}
}
