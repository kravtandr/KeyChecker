package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kravtandr/keychecker/internal/provider"
)

const maxKeys = 200

type checker interface {
	CheckAll(ctx context.Context, keys []string) []provider.Result
}

type CheckRequest struct {
	Keys []string `json:"keys"`
}

type checkResponse struct {
	Results []provider.Result `json:"results"`
}

// CheckHandler принимает пачку ключей, отбрасывает пустые строки и возвращает
// результаты в порядке ввода.
func CheckHandler(svc checker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var req CheckRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		keys := make([]string, 0, len(req.Keys))
		for _, k := range req.Keys {
			if k = strings.TrimSpace(k); k != "" {
				keys = append(keys, k)
			}
			if len(keys) >= maxKeys {
				break
			}
		}
		results := svc.CheckAll(r.Context(), keys)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(checkResponse{Results: results})
	})
}

// NewMux собирает маршрутизацию: /healthz открыт, /api/check под токеном,
// всё остальное — статика SPA.
func NewMux(token string, svc checker) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/api/check", RequireToken(token, CheckHandler(svc)))
	mux.Handle("/", StaticHandler())
	return mux
}
