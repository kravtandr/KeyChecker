package provider

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type Anthropic struct {
	baseURL string
	client  *http.Client
}

func NewAnthropic() *Anthropic {
	return &Anthropic{
		baseURL: "https://api.anthropic.com",
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *Anthropic) ID() string { return "anthropic" }

func (p *Anthropic) Matches(key string) bool {
	return strings.HasPrefix(key, "sk-ant-")
}

func (p *Anthropic) Check(ctx context.Context, key string) (Result, error) {
	res := Result{Key: Mask(key), Provider: p.ID()}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/models", nil)
	if err != nil {
		return res, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")

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
