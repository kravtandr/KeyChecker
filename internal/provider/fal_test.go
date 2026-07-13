package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFalValid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/tokens/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Key id:secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`"a.jwt.token"`))
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

func TestFalInvalid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"No user found for Key ID and Secret"}`))
	}))
	defer ts.Close()

	p := NewFal()
	p.baseURL = ts.URL

	res, _ := p.Check(context.Background(), "bad-id:bad-secret")
	if res.Valid {
		t.Fatalf("expected invalid, got %+v", res)
	}
}

func TestFalAuthPassedBadBody(t *testing.T) {
	// Валидный ключ, но тело отклонено (422) — ключ всё равно считается валидным.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Key id:secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer ts.Close()

	p := NewFal()
	p.baseURL = ts.URL

	res, _ := p.Check(context.Background(), "id:secret")
	if !res.Valid {
		t.Fatalf("expected valid (auth passed), got %+v", res)
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
