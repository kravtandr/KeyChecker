package provider

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type OpenAI struct {
	baseURL string
	client  *http.Client
}

func NewOpenAI() *OpenAI {
	return &OpenAI{
		baseURL: "https://api.openai.com",
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *OpenAI) ID() string { return "openai" }

func (p *OpenAI) Matches(key string) bool {
	return strings.HasPrefix(key, "sk-") &&
		!strings.HasPrefix(key, "sk-ant-") &&
		!strings.HasPrefix(key, "sk-or-")
}

func (p *OpenAI) Check(ctx context.Context, key string) (Result, error) {
	res := Result{Key: Mask(key), Provider: p.ID()}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/models", nil)
	if err != nil {
		return res, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

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
