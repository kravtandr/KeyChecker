package provider

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type Perplexity struct {
	baseURL string
	client  *http.Client
}

func NewPerplexity() *Perplexity {
	return &Perplexity{
		baseURL: "https://api.perplexity.ai",
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *Perplexity) ID() string { return "perplexity" }

func (p *Perplexity) Matches(key string) bool {
	return strings.HasPrefix(key, "pplx-")
}

// Check валидирует минимальным chat-запросом: у Perplexity нет бесплатного
// «только-проверить» эндпоинта. Логика опирается на статус-код.
func (p *Perplexity) Check(ctx context.Context, key string) (Result, error) {
	res := Result{Key: Mask(key), Provider: p.ID()}

	payload := strings.NewReader(`{"model":"sonar","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", payload)
	if err != nil {
		return res, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return res, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		res.Valid = true
		res.Detail = "валиден; баланс недоступен по API"
	case http.StatusUnauthorized, http.StatusForbidden:
		res.Detail = "ключ отклонён (401/403)"
	default:
		res.Detail = "неожиданный ответ провайдера"
	}
	return res, nil
}
