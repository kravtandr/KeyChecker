package provider

import "context"

// Balance описывает остаток по ключу, если провайдер отдаёт его по API.
type Balance struct {
	Amount   float64  `json:"amount"`
	Currency string   `json:"currency"`
	Limit    *float64 `json:"limit,omitempty"` // nil если лимит неизвестен
}

// Result — итог проверки одного ключа.
type Result struct {
	Key      string   `json:"key"` // всегда замаскированный, см. Mask
	Provider string   `json:"provider"`
	Valid    bool     `json:"valid"`
	Balance  *Balance `json:"balance,omitempty"`
	Detail   string   `json:"detail"`
}

// Provider — контракт для одного сервиса. Новый провайдер = один новый файл.
type Provider interface {
	ID() string
	Matches(key string) bool
	Check(ctx context.Context, key string) (Result, error)
}

// Mask скрывает секрет: первые 6 и последние 4 символа.
func Mask(key string) string {
	if len(key) < 12 {
		return "***"
	}
	return key[:6] + "..." + key[len(key)-4:]
}
