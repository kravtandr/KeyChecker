package provider

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type Fal struct {
	baseURL string
	client  *http.Client
}

func NewFal() *Fal {
	return &Fal{
		baseURL: "https://rest.alpha.fal.ai",
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *Fal) ID() string { return "fal" }

func (p *Fal) Matches(key string) bool {
	if strings.HasPrefix(key, "fal-") || strings.HasPrefix(key, "key-") {
		return true
	}
	// Формат key_id:key_secret — ровно одно двоеточие, обе части непустые,
	// и это не ключ другого провайдера.
	if strings.HasPrefix(key, "sk-") || strings.HasPrefix(key, "pplx-") {
		return false
	}
	parts := strings.Split(key, ":")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

// Check делает лёгкий авторизованный запрос: у FAL нет стабильного публичного
// «только-проверить» эндпоинта, поэтому логика опирается на статус-код.
func (p *Fal) Check(ctx context.Context, key string) (Result, error) {
	res := Result{Key: Mask(key), Provider: p.ID()}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/health", nil)
	if err != nil {
		return res, err
	}
	req.Header.Set("Authorization", "Key "+key)

	resp, err := p.client.Do(req)
	if err != nil {
		return res, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		res.Valid = true
		res.Detail = "валиден; баланс недоступен по API"
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		res.Detail = "ключ отклонён (401/403)"
	default:
		res.Detail = "неожиданный ответ провайдера"
	}
	return res, nil
}
