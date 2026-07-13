package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type OpenRouter struct {
	baseURL string
	client  *http.Client
}

func NewOpenRouter() *OpenRouter {
	return &OpenRouter{
		baseURL: "https://openrouter.ai",
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *OpenRouter) ID() string { return "openrouter" }

func (p *OpenRouter) Matches(key string) bool {
	return strings.HasPrefix(key, "sk-or-")
}

type openRouterKeyResp struct {
	Data struct {
		Usage          float64  `json:"usage"`
		Limit          *float64 `json:"limit"`
		LimitRemaining *float64 `json:"limit_remaining"`
	} `json:"data"`
}

func (p *OpenRouter) Check(ctx context.Context, key string) (Result, error) {
	res := Result{Key: Mask(key), Provider: p.ID()}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/v1/key", nil)
	if err != nil {
		return res, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := p.client.Do(req)
	if err != nil {
		return res, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		res.Detail = "ключ отклонён (401/403)"
		return res, nil
	}
	if resp.StatusCode != http.StatusOK {
		res.Detail = "неожиданный ответ провайдера"
		return res, nil
	}

	var body openRouterKeyResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		res.Valid = true
		res.Detail = "валиден; не удалось разобрать баланс"
		return res, nil
	}

	res.Valid = true
	amount := body.Data.Usage
	if body.Data.LimitRemaining != nil {
		amount = *body.Data.LimitRemaining
	}
	res.Balance = &Balance{Amount: amount, Currency: "USD", Limit: body.Data.Limit}
	res.Detail = "валиден; баланс из /api/v1/key"
	return res, nil
}
