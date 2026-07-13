package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFalValid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Key id:secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	p := NewFal()
	p.baseURL = ts.URL

	res, err := p.Check(context.Background(), "id:secret")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Provider != "fal" {
		t.Fatalf("expected valid fal, got %+v", res)
	}
}

func TestFalMatches(t *testing.T) {
	if !NewFal().Matches("abcd1234:efgh5678") {
		t.Fatal("should match id:secret form")
	}
	if NewFal().Matches("sk-ant-abc") {
		t.Fatal("should not match anthropic key")
	}
}
