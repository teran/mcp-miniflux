package miniflux

// Supplementary redaction tests by @developer to raise coverage of the
// reflection helper beyond QA's locked contract tests.

import (
	"testing"

	dmf "github.com/teran/mcp-miniflux/domain/miniflux"
)

// secretViaInterface holds a secret nested behind an interface-typed field so
// Redact must recurse through the interface kind (S02).
type secretViaInterface struct {
	Label string `json:"label"`
	Thing any    `json:"thing"`
}

// unexportedFieldStruct has an unexported field that must be skipped, plus an
// exported secret that must be zeroed.
type unexportedFieldStruct struct {
	hidden string
	Token  string `json:"token" secret:"true"`
	Keep   string `json:"keep"`
}

func TestRedactRecursesThroughInterface(t *testing.T) {
	inner := dmf.Feed{ID: 1, Title: "T", Password: "p"}
	o := secretViaInterface{Label: "l", Thing: inner}

	got, ok := Redact(o).(secretViaInterface)
	if !ok {
		t.Fatalf("Redact = %T, want secretViaInterface", Redact(o))
	}
	if got.Label != "l" {
		t.Errorf("Label = %q, want l", got.Label)
	}
	f, ok := got.Thing.(dmf.Feed)
	if !ok {
		t.Fatalf("Thing = %T, want dmf.Feed", got.Thing)
	}
	if f.Password != "" || f.ID != 1 {
		t.Errorf("nested-in-interface secret not handled: %+v", f)
	}
	if inner.Password != "p" {
		t.Errorf("Redact mutated original: %+v", inner)
	}
}

func TestRedactSkipsUnexportedField(t *testing.T) {
	o := unexportedFieldStruct{hidden: "sensitive", Token: "t", Keep: "k"}
	got, ok := Redact(o).(unexportedFieldStruct)
	if !ok {
		t.Fatalf("Redact = %T", Redact(o))
	}
	if got.Token != "" || got.Keep != "k" {
		t.Errorf("Redact = %+v, want Token zeroed, Keep kept", got)
	}
	if o.Token != "t" {
		t.Errorf("Redact mutated original: %+v", o)
	}
}

func TestRedactNonNilChanAndFuncPassThrough(t *testing.T) {
	ch := make(chan int)
	if got, ok := Redact(ch).(chan int); !ok || got == nil {
		t.Errorf("Redact(chan) = %#v", Redact(ch))
	}
	f := func() {}
	if got, ok := Redact(f).(func()); !ok || got == nil {
		t.Errorf("Redact(func) = %#v", Redact(f))
	}
}

func TestRedactNilMapReturnsNil(t *testing.T) {
	var m map[string]dmf.Feed
	if got := Redact(m); got != nil {
		t.Errorf("Redact(nil map) = %#v, want nil", got)
	}
}

func TestRedactNilSliceReturnsNil(t *testing.T) {
	var s []dmf.Feed
	if got := Redact(s); got != nil {
		t.Errorf("Redact(nil slice) = %#v, want nil", got)
	}
}

func TestRedactPointerToSlice(t *testing.T) {
	feeds := []dmf.Feed{{ID: 1, Password: "p"}}
	got, ok := Redact(&feeds).(*[]dmf.Feed)
	if !ok {
		t.Fatalf("Redact(&slice) = %T", Redact(&feeds))
	}
	if (*got)[0].Password != "" {
		t.Errorf("pointer-to-slice secret not zeroed: %+v", *got)
	}
	if feeds[0].Password != "p" {
		t.Errorf("Redact mutated original: %+v", feeds)
	}
}
