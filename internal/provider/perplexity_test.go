package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPerplexityValid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer pplx-good" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi"}}]}`))
	}))
	defer ts.Close()

	p := NewPerplexity()
	p.baseURL = ts.URL

	res, err := p.Check(context.Background(), "pplx-good")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Provider != "perplexity" {
		t.Fatalf("expected valid perplexity, got %+v", res)
	}
}

func TestPerplexityInvalid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	p := NewPerplexity()
	p.baseURL = ts.URL

	res, _ := p.Check(context.Background(), "pplx-bad")
	if res.Valid {
		t.Fatal("expected invalid")
	}
}
