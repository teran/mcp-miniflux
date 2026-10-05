package miniflux

import (
	"reflect"
	"testing"
)

// redactInner / redactOuter are local fixtures exercising deep recursion
// (nested structs, pointers, slices, maps and interfaces) without touching
// production types.
type redactInner struct {
	Pwd string `secret:"true"`
	OK  string
}

type redactOuter struct {
	Inner  redactInner
	Ptr    *redactInner
	Slice  []redactInner
	Map    map[string]redactInner
	Any    any
	Secret string `secret:"true"`
	Plain  string
	Nested redactInner
}

// redactUnexported covers the unexported-field skip branch in redactStruct.
type redactUnexported struct {
	hidden string `secret:"true"`
	Public string `secret:"true"`
}

// --- Redact: happy path on production types -------------------------------

func TestRedactFeedZeroesSecretsAndPreservesRest(t *testing.T) {
	in := Feed{
		ID:         42,
		Title:      "Example",
		Username:   "svc",
		Password:   "super-secret",
		Category:   CategoryRef{ID: 3, Title: "Tech"},
		ErrorCount: 2,
	}

	got := Redact(in)
	out, ok := got.(Feed)
	if !ok {
		t.Fatalf("Redact(Feed) returned %T, want Feed", got)
	}
	if out.Password != "" || out.Username != "" {
		t.Errorf("secrets not zeroed: %q/%q", out.Username, out.Password)
	}
	if out.ID != 42 || out.Title != "Example" || out.ErrorCount != 2 {
		t.Errorf("non-secret fields dropped: %+v", out)
	}
	if out.Category != (CategoryRef{ID: 3, Title: "Tech"}) {
		t.Errorf("nested Category changed: %+v", out.Category)
	}

	// Input must not be mutated (deep copy).
	if in.Password != "super-secret" || in.Username != "svc" {
		t.Errorf("Redact mutated input: %+v", in)
	}
}

func TestRedactCreateFeedRequest(t *testing.T) {
	cat := 3
	in := CreateFeedRequest{FeedURL: "https://x/rss", CategoryID: &cat, Title: "T", Username: "u", Password: "p"}
	out := Redact(in).(CreateFeedRequest)
	if out.Password != "" || out.Username != "" {
		t.Errorf("CreateFeedRequest secrets not zeroed: %+v", out)
	}
	if out.FeedURL != "https://x/rss" || out.Title != "T" || out.CategoryID == nil || *out.CategoryID != 3 {
		t.Errorf("CreateFeedRequest non-secrets changed: %+v", out)
	}
	if in.Password != "p" {
		t.Errorf("Redact mutated CreateFeedRequest input")
	}
}

func TestRedactUpdateFeedRequest(t *testing.T) {
	crawler := true
	in := UpdateFeedRequest{Title: "T", SiteURL: "https://x", Username: "u", Password: "p", Crawler: &crawler}
	out := Redact(in).(UpdateFeedRequest)
	if out.Password != "" || out.Username != "" {
		t.Errorf("UpdateFeedRequest secrets not zeroed: %+v", out)
	}
	if out.Title != "T" || out.SiteURL != "https://x" || out.Crawler == nil || !*out.Crawler {
		t.Errorf("UpdateFeedRequest non-secrets changed: %+v", out)
	}
	if in.Password != "p" {
		t.Errorf("Redact mutated UpdateFeedRequest input")
	}
}

// --- Redact: deep recursion ------------------------------------------------

func TestRedactDeepCopyRecurses(t *testing.T) {
	inner := redactInner{Pwd: "pw", OK: "ok"}
	in := redactOuter{
		Inner:  inner,
		Ptr:    &redactInner{Pwd: "p1", OK: "k1"},
		Slice:  []redactInner{{Pwd: "p2", OK: "k2"}, {Pwd: "p3", OK: "k3"}},
		Map:    map[string]redactInner{"a": {Pwd: "p4", OK: "k4"}},
		Any:    redactInner{Pwd: "p5", OK: "k5"},
		Secret: "top",
		Plain:  "plain",
		Nested: redactInner{Pwd: "p6", OK: "k6"},
	}

	out := Redact(in).(redactOuter)

	if out.Inner.Pwd != "" || out.Nested.Pwd != "" || out.Secret != "" {
		t.Errorf("secret fields not zeroed: %+v", out)
	}
	if out.Inner.OK != "ok" || out.Nested.OK != "k6" || out.Plain != "plain" {
		t.Errorf("non-secret fields changed: %+v", out)
	}

	// Pointer recursion.
	if out.Ptr == nil {
		t.Fatal("Redact dropped non-nil pointer")
	}
	if out.Ptr.Pwd != "" || out.Ptr.OK != "k1" {
		t.Errorf("pointer not deep-copied/redacted: %+v", out.Ptr)
	}

	// Slice recursion (length preserved, elements redacted).
	if len(out.Slice) != 2 {
		t.Fatalf("slice length = %d, want 2", len(out.Slice))
	}
	for i, e := range out.Slice {
		if e.Pwd != "" || e.OK == "" {
			t.Errorf("slice elem %d not redacted: %+v", i, e)
		}
	}

	// Map recursion.
	if len(out.Map) != 1 {
		t.Fatalf("map length = %d, want 1", len(out.Map))
	}
	mv := out.Map["a"]
	if mv.Pwd != "" || mv.OK != "k4" {
		t.Errorf("map value not redacted: %+v", mv)
	}

	// Interface recursion.
	anyVal, ok := out.Any.(redactInner)
	if !ok {
		t.Fatalf("Any = %T, want redactInner", out.Any)
	}
	if anyVal.Pwd != "" || anyVal.OK != "k5" {
		t.Errorf("interface value not redacted: %+v", anyVal)
	}

	// Deep copy: mutating the result must not affect the input, and vice versa.
	out.Plain = "mutated"
	out.Ptr.OK = "mutated"
	if in.Plain != "plain" || in.Ptr.OK != "k1" {
		t.Errorf("Redact returned a shallow copy (aliased to input)")
	}
}

func TestRedactPointerNonNil(t *testing.T) {
	in := &Feed{ID: 1, Password: "secret"}
	out, ok := Redact(in).(*Feed)
	if !ok {
		t.Fatalf("Redact(*Feed) = %T, want *Feed", Redact(in))
	}
	if out == nil || out.Password != "" || out.ID != 1 {
		t.Errorf("pointer redaction wrong: %+v", out)
	}
	if in.Password != "secret" {
		t.Errorf("input pointer mutated")
	}
}

func TestRedactSliceAndMapNonNil(t *testing.T) {
	feeds := []Feed{{ID: 1, Password: "a"}, {ID: 2, Password: "b"}}
	out := Redact(feeds).([]Feed)
	if len(out) != 2 || out[0].Password != "" || out[1].Password != "" || out[0].ID != 1 {
		t.Errorf("slice redaction wrong: %+v", out)
	}
	if feeds[0].Password != "a" {
		t.Errorf("input slice mutated")
	}

	m := map[string]Feed{"k": {ID: 9, Password: "p"}}
	mo := Redact(m).(map[string]Feed)
	if mo["k"].Password != "" || mo["k"].ID != 9 {
		t.Errorf("map redaction wrong: %+v", mo)
	}
	if m["k"].Password != "p" {
		t.Errorf("input map mutated")
	}
}

// --- Redact: nil handling ----------------------------------------------------

func TestRedactNilReturnsNil(t *testing.T) {
	if got := Redact(nil); got != nil {
		t.Errorf("Redact(nil) = %v, want nil", got)
	}
}

func TestRedactTypedNilBecomesUntypedNil(t *testing.T) {
	// A typed-nil interface must collapse to an untyped nil.
	if got := Redact((*int)(nil)); got != nil {
		t.Errorf("Redact((*int)(nil)) = %#v, want nil", got)
	}
	if got := Redact([]Feed(nil)); got != nil {
		t.Errorf("Redact(nil slice) = %#v, want nil", got)
	}
	if got := Redact(map[string]Feed(nil)); got != nil {
		t.Errorf("Redact(nil map) = %#v, want nil", got)
	}
}

// --- Redact: non-struct passthrough ------------------------------------------

func TestRedactNonStructPassthrough(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
	}{
		{"string", "hello"},
		{"int", 42},
		{"bool", true},
		{"float", 3.14},
		{"int-slice", []int{1, 2, 3}},
	} {
		if got := Redact(tc.in); !reflect.DeepEqual(got, tc.in) {
			t.Errorf("Redact(%s) = %#v, want unchanged %#v", tc.name, got, tc.in)
		}
	}
}

func TestRedactEmptyInterfaceFieldAndUnexportedField(t *testing.T) {
	// Nil interface field must not panic and stays nil.
	in := redactOuter{Any: nil}
	out := Redact(in).(redactOuter)
	if out.Any != nil {
		t.Errorf("nil interface field changed: %#v", out.Any)
	}

	// Unexported secret fields are skipped (left zero), exported are zeroed.
	u := redactUnexported{hidden: "h", Public: "p"}
	uo := Redact(u).(redactUnexported)
	if uo.Public != "" {
		t.Errorf("exported secret field not zeroed: %+v", uo)
	}
	// hidden is unexported -> skipped; out is a fresh zero copy so it is "".
	if uo.hidden != "" {
		t.Errorf("unexported field leaked: %q", uo.hidden)
	}
	if u.Public != "p" {
		t.Errorf("input mutated")
	}
}

// redactValue with an invalid reflect.Value short-circuits at the
// `!v.IsValid()` guard.
func TestRedactValueInvalidValue(t *testing.T) {
	got := redactValue(reflect.Value{})
	if got.IsValid() {
		t.Errorf("redactValue(invalid) = %v, want invalid value", got)
	}
}

// --- isNilValue direct coverage ---------------------------------------------

func TestIsNilValue(t *testing.T) {
	if !isNilValue(reflect.Value{}) {
		t.Error("isNilValue(invalid) = false, want true")
	}
	if !isNilValue(reflect.ValueOf((*int)(nil))) {
		t.Error("isNilValue(nil pointer) = false")
	}
	var ch chan int
	if !isNilValue(reflect.ValueOf(ch)) {
		t.Error("isNilValue(nil chan) = false")
	}
	var fn func()
	if !isNilValue(reflect.ValueOf(fn)) {
		t.Error("isNilValue(nil func) = false")
	}
	var m map[string]int
	if !isNilValue(reflect.ValueOf(m)) {
		t.Error("isNilValue(nil map) = false")
	}
	var s []int
	if !isNilValue(reflect.ValueOf(s)) {
		t.Error("isNilValue(nil slice) = false")
	}
	// A nil interface-kind value (via a pointer deref) is nil-able.
	var iface any
	if !isNilValue(reflect.ValueOf(&iface).Elem()) {
		t.Error("isNilValue(nil interface) = false")
	}

	// Non-nil / non-nilable kinds are false.
	if isNilValue(reflect.ValueOf("x")) {
		t.Error("isNilValue(string) = true")
	}
	if isNilValue(reflect.ValueOf(&struct{}{})) {
		t.Error("isNilValue(non-nil pointer) = true")
	}
}

// --- RedactMap ---------------------------------------------------------------

func TestRedactMapRemovesSecretKeysCaseInsensitive(t *testing.T) {
	in := map[string]any{
		"password":  "pw",
		"Password":  "pw2",
		"token":     "tok",
		"API_TOKEN": "api",
		"apikey":    "key",
		"ApiKey":    "key2",
		"title":     "T",
		"url":       "https://x",
	}
	out := RedactMap(in)
	for _, k := range []string{"password", "Password", "token", "API_TOKEN", "apikey", "ApiKey"} {
		if _, ok := out[k]; ok {
			t.Errorf("secret key %q not removed: %v", k, out)
		}
	}
	if out["title"] != "T" || out["url"] != "https://x" {
		t.Errorf("non-secret keys changed/dropped: %v", out)
	}
	if len(out) != 2 {
		t.Errorf("out length = %d, want 2", len(out))
	}

	// Input never mutated.
	if in["password"] != "pw" || in["API_TOKEN"] != "api" || in["title"] != "T" {
		t.Errorf("RedactMap mutated input: %v", in)
	}
}

func TestRedactMapKeepsOtherKeys(t *testing.T) {
	in := map[string]any{"secret-note": "x", "passwordX": "not-secret"}
	out := RedactMap(in)
	if out["secret-note"] != "x" {
		t.Errorf("non-secret keys dropped: %v", out)
	}
	// passwordX lowercases to "passwordx" which is NOT in the set -> kept.
	if _, ok := out["passwordX"]; !ok {
		t.Errorf("passwordX should be kept")
	}
}

func TestRedactMapRecursesNested(t *testing.T) {
	in := map[string]any{
		"nested": map[string]any{
			"password": "pw",
			"deep":     map[string]any{"token": "t", "keep": "k"},
			"items":    []any{map[string]any{"apikey": "a", "ok": "o"}, "str", 7},
		},
		"top": "v",
	}
	out := RedactMap(in)

	nested, ok := out["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested = %T, want map[string]any", out["nested"])
	}
	if _, ok := nested["password"]; ok {
		t.Errorf("nested password not removed: %v", nested)
	}
	deep := nested["deep"].(map[string]any)
	if _, ok := deep["token"]; ok {
		t.Errorf("deep token not removed: %v", deep)
	}
	if deep["keep"] != "k" {
		t.Errorf("deep keep dropped: %v", deep)
	}

	items := nested["items"].([]any)
	first := items[0].(map[string]any)
	if _, ok := first["apikey"]; ok {
		t.Errorf("slice-item apikey not removed: %v", first)
	}
	if first["ok"] != "o" {
		t.Errorf("slice-item ok dropped: %v", first)
	}
	if items[1] != "str" || items[2] != 7 {
		t.Errorf("non-map slice items changed: %v", items)
	}

	if out["top"] != "v" {
		t.Errorf("top changed: %v", out)
	}

	// Input not mutated.
	if in["nested"].(map[string]any)["password"] != "pw" {
		t.Errorf("RedactMap mutated input")
	}
}

// CONFORM-AUDIT [S02] — `username` is a feed HTTP-basic-auth credential
// (CreateFeedRequest/UpdateFeedRequest carry it as `secret:"true"`). It must
// therefore be redacted from arbitrary access-log / tool-output maps by
// RedactMap, exactly like `password`. This test locks that adding "username"
// (and case variants) to the secret-key set is required.
//
// NOTE: this intentionally supersedes the assertion in
// TestRedactMapKeepsOtherKeys that treated "username" as a non-secret key —
// the audit classified the feed `username` credential as a secret, so that
// older expectation is being corrected (the developer must update
// TestRedactMapKeepsOtherKeys to drop its `username` keep-assertion).
func TestRedactMapRemovesUsername(t *testing.T) {
	in := map[string]any{
		"username": "alice",
		"Username": "alice2",
		"USERNAME": "alice3",
		"title":    "ok",
	}
	out := RedactMap(in)
	for _, k := range []string{"username", "Username", "USERNAME"} {
		if _, ok := out[k]; ok {
			t.Errorf("secret key %q not removed: %v (S02)", k, out)
		}
	}
	if out["title"] != "ok" {
		t.Errorf("non-secret key dropped: %v", out)
	}
	if in["username"] != "alice" {
		t.Errorf("RedactMap mutated input: %v", in)
	}
}

// TestRedactMapRecursesUsernameRemoval locks that username is also stripped
// inside nested maps and slices (the access-log `args` may nest credentials).
func TestRedactMapRecursesUsernameRemoval(t *testing.T) {
	in := map[string]any{
		"creds": map[string]any{"username": "bob", "keep": 1},
		"list":  []any{map[string]any{"username": "carol", "ok": "x"}},
	}
	out := RedactMap(in)

	creds := out["creds"].(map[string]any)
	if _, ok := creds["username"]; ok {
		t.Errorf("nested username not removed: %v (S02)", creds)
	}
	if creds["keep"] != 1 {
		t.Errorf("nested non-secret dropped: %v", creds)
	}
	list := out["list"].([]any)
	first := list[0].(map[string]any)
	if _, ok := first["username"]; ok {
		t.Errorf("slice-item username not removed: %v (S02)", first)
	}
	if first["ok"] != "x" {
		t.Errorf("slice-item non-secret dropped: %v", first)
	}
}

func TestRedactMapNil(t *testing.T) {
	if got := RedactMap(nil); got != nil {
		t.Errorf("RedactMap(nil) = %v, want nil", got)
	}
}

func TestRedactMapNonMapValuesUnchanged(t *testing.T) {
	in := map[string]any{"a": 1, "b": "x", "c": []int{1, 2}}
	out := RedactMap(in)
	if out["a"] != 1 || out["b"] != "x" {
		t.Errorf("scalar values changed: %v", out)
	}
	// []int is not []any -> forwarded unchanged.
	if !reflect.DeepEqual(out["c"], []int{1, 2}) {
		t.Errorf("[]int value changed: %#v", out["c"])
	}
}
