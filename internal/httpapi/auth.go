package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// RequireToken защищает next общим Bearer-токеном. Пустой токен — ошибка
// конфигурации: всегда 500 (основной fail-closed выполняется в main).
func RequireToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			http.Error(w, `{"error":"server misconfigured"}`, http.StatusInternalServerError)
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
