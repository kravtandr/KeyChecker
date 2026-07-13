package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIValid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o"}]}`))
	}))
	defer ts.Close()

	p := NewOpenAI()
	p.baseURL = ts.URL

	res, err := p.Check(context.Background(), "sk-good")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Provider != "openai" {
		t.Fatalf("expected valid openai, got %+v", res)
	}
	if res.Key == "sk-good" {
		t.Fatalf("key must be masked, got %q", res.Key)
	}
}

func TestOpenAIInvalid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	p := NewOpenAI()
	p.baseURL = ts.URL

	res, _ := p.Check(context.Background(), "sk-bad")
	if res.Valid {
		t.Fatalf("expected invalid")
	}
}

func TestOpenAIMatches(t *testing.T) {
	p := NewOpenAI()
	if !p.Matches("sk-proj-abc") {
		t.Fatal("should match sk-")
	}
	if p.Matches("sk-ant-abc") || p.Matches("sk-or-v1-abc") {
		t.Fatal("should not match anthropic/openrouter")
	}
}
