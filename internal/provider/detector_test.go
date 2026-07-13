package provider

import (
	"context"
	"testing"
)

type fakeProvider struct{ id, prefix string }

func (f fakeProvider) ID() string { return f.id }
func (f fakeProvider) Matches(k string) bool {
	return len(k) >= len(f.prefix) && k[:len(f.prefix)] == f.prefix
}
func (f fakeProvider) Check(context.Context, string) (Result, error) {
	return Result{Provider: f.id, Valid: true}, nil
}

func TestDetect(t *testing.T) {
	ps := []Provider{fakeProvider{"a", "aa-"}, fakeProvider{"b", "bb-"}}

	if got := Detect(ps, "bb-123"); got == nil || got.ID() != "b" {
		t.Fatalf("expected provider b")
	}
	if got := Detect(ps, "zz-123"); got != nil {
		t.Fatalf("expected nil for unknown prefix")
	}
}
