package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenRouterValidWithBalance(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-or-good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"label":"key","usage":3.5,"limit":10,"limit_remaining":6.5}}`))
	}))
	defer ts.Close()

	p := NewOpenRouter()
	p.baseURL = ts.URL

	res, err := p.Check(context.Background(), "sk-or-good")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Fatalf("expected valid, got %+v", res)
	}
	if res.Balance == nil || res.Balance.Amount != 6.5 {
		t.Fatalf("expected balance 6.5, got %+v", res.Balance)
	}
	if res.Balance.Limit == nil || *res.Balance.Limit != 10 {
		t.Fatalf("expected limit 10, got %+v", res.Balance)
	}
}

func TestOpenRouterMatches(t *testing.T) {
	if !NewOpenRouter().Matches("sk-or-v1-abc") {
		t.Fatal("should match sk-or-")
	}
}
