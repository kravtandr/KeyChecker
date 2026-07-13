package check

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kravtandr/keychecker/internal/provider"
)

const (
	maxParallel = 8
	perKeyTO    = 20 * time.Second
)

// DefaultProviders — порядок важен: специфичные префиксы раньше общего sk-.
func DefaultProviders() []provider.Provider {
	return []provider.Provider{
		provider.NewAnthropic(),
		provider.NewOpenRouter(),
		provider.NewOpenAI(),
		provider.NewPerplexity(),
		provider.NewFal(),
	}
}

type Service struct {
	providers []provider.Provider
}

func NewService(ps []provider.Provider) *Service {
	return &Service{providers: ps}
}

// CheckAll параллельно проверяет ключи (лимит maxParallel), сохраняя порядок ввода.
func (s *Service) CheckAll(ctx context.Context, keys []string) []provider.Result {
	results := make([]provider.Result, len(keys))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxParallel)

	for i, key := range keys {
		i, key := i, key
		g.Go(func() error {
			results[i] = s.checkOne(ctx, key)
			return nil
		})
	}
	_ = g.Wait()
	return results
}

func (s *Service) checkOne(ctx context.Context, key string) provider.Result {
	p := provider.Detect(s.providers, key)
	if p == nil {
		return provider.Result{
			Key:      provider.Mask(key),
			Provider: "unknown",
			Valid:    false,
			Detail:   "провайдер не распознан",
		}
	}

	cctx, cancel := context.WithTimeout(ctx, perKeyTO)
	defer cancel()

	res, err := p.Check(cctx, key)
	if err != nil {
		return provider.Result{
			Key:      provider.Mask(key),
			Provider: p.ID(),
			Valid:    false,
			Detail:   "ошибка запроса: " + err.Error(),
		}
	}
	return res
}
