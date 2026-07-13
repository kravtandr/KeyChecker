package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kravtandr/keychecker/internal/provider"
)

type fakeChecker struct{}

func (fakeChecker) CheckAll(_ context.Context, keys []string) []provider.Result {
	out := make([]provider.Result, len(keys))
	for i, k := range keys {
		out[i] = provider.Result{Key: provider.Mask(k), Provider: "openai", Valid: true}
	}
	return out
}

func TestCheckHandler(t *testing.T) {
	h := CheckHandler(fakeChecker{})
	body := `{"keys":["sk-aaaaaaaaaaaa"," ","sk-bbbbbbbbbbbb"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/check", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rr.Code)
	}
	var resp struct {
		Results []provider.Result `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("blank line must be dropped, got %d results", len(resp.Results))
	}
}

func TestCheckHandlerRejectsGet(t *testing.T) {
	h := CheckHandler(fakeChecker{})
	req := httptest.NewRequest(http.MethodGet, "/api/check", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", rr.Code)
	}
}
