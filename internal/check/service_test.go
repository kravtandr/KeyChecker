package check

import (
	"context"
	"testing"

	"github.com/kravtandr/keychecker/internal/provider"
)

type stubProvider struct {
	id, prefix string
	valid      bool
}

func (s stubProvider) ID() string { return s.id }
func (s stubProvider) Matches(k string) bool {
	return len(k) >= len(s.prefix) && k[:len(s.prefix)] == s.prefix
}
func (s stubProvider) Check(_ context.Context, k string) (provider.Result, error) {
	return provider.Result{Key: provider.Mask(k), Provider: s.id, Valid: s.valid}, nil
}

func TestCheckAllOrderAndUnknown(t *testing.T) {
	ps := []provider.Provider{
		stubProvider{"a", "aa-", true},
		stubProvider{"b", "bb-", false},
	}
	svc := NewService(ps)

	keys := []string{"aa-1111111111", "zz-9999999999", "bb-2222222222"}
	got := svc.CheckAll(context.Background(), keys)

	if len(got) != 3 {
		t.Fatalf("want 3 results, got %d", len(got))
	}
	if got[0].Provider != "a" || !got[0].Valid {
		t.Errorf("result 0 wrong: %+v", got[0])
	}
	if got[1].Provider != "unknown" || got[1].Valid {
		t.Errorf("result 1 should be unknown: %+v", got[1])
	}
	if got[2].Provider != "b" || got[2].Valid {
		t.Errorf("result 2 wrong: %+v", got[2])
	}
}
