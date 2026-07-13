package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicValid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-ant-good" || r.Header.Get("anthropic-version") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-3-5-sonnet"}]}`))
	}))
	defer ts.Close()

	p := NewAnthropic()
	p.baseURL = ts.URL

	res, err := p.Check(context.Background(), "sk-ant-good")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Provider != "anthropic" {
		t.Fatalf("expected valid anthropic, got %+v", res)
	}
}

func TestAnthropicMatches(t *testing.T) {
	if !NewAnthropic().Matches("sk-ant-api03-x") {
		t.Fatal("should match sk-ant-")
	}
	if NewAnthropic().Matches("sk-proj-x") {
		t.Fatal("should not match plain sk-")
	}
}
