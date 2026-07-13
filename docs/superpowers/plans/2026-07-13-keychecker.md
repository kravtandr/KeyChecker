# KeyChecker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Self-hosted веб-сервис, который проверяет пачку API-ключей LLM-провайдеров на валидность и показывает баланс там, где API это отдаёт.

**Architecture:** Go-бэкенд (`net/http`) с интерфейсом `Provider` на каждый сервис и фан-аут-проверкой через `errgroup`. React/Vite SPA отдаётся тем же Go-бинарником через `embed`. Ключи stateless — только в памяти на время запроса. Доступ защищён общим Bearer-токеном из env (fail-closed).

**Tech Stack:** Go 1.23, `golang.org/x/sync/errgroup`, стандартный `net/http` + `net/http/httptest` для тестов; React 18 + Vite + TypeScript; Docker (multi-stage).

## Global Constraints

- Module path: `github.com/kravtandr/keychecker`.
- Ключи НИКОГДА не логируются, не пишутся на диск и не кэшируются; в любом выводе маскируются функцией `Mask`.
- Внешние запросы идут только на захардкоженные HTTPS-хосты провайдеров (нет URL, управляемых пользователем) — это и есть защита от SSRF.
- Bearer-токен читается из env `KEYCHECKER_TOKEN`; при пустом токене сервис не стартует (fail-closed). Токен не логируется.
- Результаты возвращаются строго в порядке ввода ключей.
- Каждый провайдер тестируется на замоканных HTTP-ответах (`httptest`), без реальной сети и реальных ключей.
- Go ставится локально в `$HOME/.local/go` (sudo недоступен); `export PATH=$HOME/.local/go/bin:$PATH` нужен в каждой сессии сборки.

---

### Task 1: Окружение и скелет бэкенда

**Files:**
- Create: `go.mod`
- Create: `cmd/keychecker/main.go`
- Create: `cmd/keychecker/main_test.go`
- Create: `.gitignore`

**Interfaces:**
- Consumes: ничего.
- Produces: рабочий HTTP-сервер с `GET /healthz` → 200 `ok`; функция `newMux() http.Handler` для тестов.

- [ ] **Step 1: Установить Go локально**

```bash
curl -sL https://go.dev/dl/go1.23.5.linux-amd64.tar.gz -o /tmp/go.tar.gz
rm -rf "$HOME/.local/go" && mkdir -p "$HOME/.local" && tar -C "$HOME/.local" -xzf /tmp/go.tar.gz
export PATH="$HOME/.local/go/bin:$PATH"
go version
```
Expected: `go version go1.23.5 linux/amd64`

- [ ] **Step 2: Инициализировать модуль и .gitignore**

```bash
cd /home/kravtandr/proj/KeyChecker
go mod init github.com/kravtandr/keychecker
```
`.gitignore`:
```
/keychecker
/web/node_modules
/web/dist
/tmp
```

- [ ] **Step 3: Написать падающий тест `main_test.go`**

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	srv := httptest.NewServer(newMux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}
}
```

- [ ] **Step 4: Запустить тест — убедиться, что не компилируется/падает**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./cmd/keychecker/`
Expected: FAIL — `undefined: newMux`

- [ ] **Step 5: Реализовать `main.go`**

```go
package main

import (
	"log"
	"net/http"
	"os"
)

func newMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func main() {
	addr := os.Getenv("KEYCHECKER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("keychecker listening on %s", addr)
	if err := http.ListenAndServe(addr, newMux()); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 6: Запустить тест — убедиться, что проходит**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./cmd/keychecker/`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add go.mod cmd/keychecker/ .gitignore
git commit -m "feat: bootstrap Go module and health endpoint"
```

---

### Task 2: Типы Provider и маскирование ключей

**Files:**
- Create: `internal/provider/provider.go`
- Create: `internal/provider/mask_test.go`

**Interfaces:**
- Consumes: ничего.
- Produces:
  - `type Balance struct { Amount float64; Currency string; Limit *float64 }`
  - `type Result struct { Key string; Provider string; Valid bool; Balance *Balance; Detail string }`
  - `type Provider interface { ID() string; Matches(key string) bool; Check(ctx context.Context, key string) (Result, error) }`
  - `func Mask(key string) string` — первые 6 и последние 4 символа, середина `...`; короткие ключи → `***`.

- [ ] **Step 1: Написать падающий тест `mask_test.go`**

```go
package provider

import "testing"

func TestMask(t *testing.T) {
	cases := map[string]string{
		"sk-ant-api03-ABCDEF1234567890": "sk-ant...7890",
		"short":                         "***",
		"":                              "***",
	}
	for in, want := range cases {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/`
Expected: FAIL — `undefined: Mask`

- [ ] **Step 3: Реализовать `provider.go`**

```go
package provider

import "context"

// Balance описывает остаток по ключу, если провайдер отдаёт его по API.
type Balance struct {
	Amount   float64
	Currency string
	Limit    *float64 // nil если лимит неизвестен
}

// Result — итог проверки одного ключа.
type Result struct {
	Key      string // всегда замаскированный, см. Mask
	Provider string
	Valid    bool
	Balance  *Balance
	Detail   string
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
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/provider.go internal/provider/mask_test.go
git commit -m "feat: add Provider interface, Result types and key masking"
```

---

### Task 3: Детектор провайдера по префиксу

**Files:**
- Create: `internal/provider/detector.go`
- Create: `internal/provider/detector_test.go`

**Interfaces:**
- Consumes: `Provider` из Task 2.
- Produces: `func Detect(providers []Provider, key string) Provider` — первый `Provider`, чей `Matches(key)` истинен, иначе `nil`. Используется реестром в Task 9.

- [ ] **Step 1: Написать падающий тест `detector_test.go`**

```go
package provider

import (
	"context"
	"testing"
)

type fakeProvider struct{ id, prefix string }

func (f fakeProvider) ID() string            { return f.id }
func (f fakeProvider) Matches(k string) bool { return len(k) >= len(f.prefix) && k[:len(f.prefix)] == f.prefix }
func (f fakeProvider) Check(context.Context, string) (Result, error) {
	return Result{Provider: f.id, Valid: true}, nil
}

func TestDetect(t *testing.T) {
	ps := []Provider{fakeProvider{"a", "aa-"}, fakeProvider{"b", "bb-"}}

	if got := Detect(ps, "bb-123"); got == nil || got.ID() != "b" {
		t.Fatalf("expected provider b")
	}
	if got := Detect(ps, "zz-123"); got != nil {
		t.Fatalf("expected nil for unknown prefix")
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestDetect`
Expected: FAIL — `undefined: Detect`

- [ ] **Step 3: Реализовать `detector.go`**

```go
package provider

// Detect возвращает первый провайдер, распознавший ключ по префиксу.
func Detect(providers []Provider, key string) Provider {
	for _, p := range providers {
		if p.Matches(key) {
			return p
		}
	}
	return nil
}
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestDetect`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/detector.go internal/provider/detector_test.go
git commit -m "feat: add provider detector by key prefix"
```

---

### Task 4: Провайдер OpenAI

**Files:**
- Create: `internal/provider/openai.go`
- Create: `internal/provider/openai_test.go`

**Interfaces:**
- Consumes: `Provider`, `Result`, `Mask` из Task 2.
- Produces: `func NewOpenAI() *OpenAI`; поля `baseURL string`, `client *http.Client` (переопределяются в тестах). `ID()=="openai"`, `Matches` истинен для `sk-` (кроме `sk-ant-` и `sk-or-`). Баланс всегда `nil`.

- [ ] **Step 1: Написать падающий тест `openai_test.go`**

```go
package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIValid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o"}]}`))
	}))
	defer ts.Close()

	p := NewOpenAI()
	p.baseURL = ts.URL

	res, err := p.Check(context.Background(), "sk-good")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Provider != "openai" {
		t.Fatalf("expected valid openai, got %+v", res)
	}
	if res.Key != "***" && res.Key == "sk-good" {
		t.Fatalf("key must be masked, got %q", res.Key)
	}
}

func TestOpenAIInvalid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	p := NewOpenAI()
	p.baseURL = ts.URL

	res, _ := p.Check(context.Background(), "sk-bad")
	if res.Valid {
		t.Fatalf("expected invalid")
	}
}

func TestOpenAIMatches(t *testing.T) {
	p := NewOpenAI()
	if !p.Matches("sk-proj-abc") {
		t.Fatal("should match sk-")
	}
	if p.Matches("sk-ant-abc") || p.Matches("sk-or-v1-abc") {
		t.Fatal("should not match anthropic/openrouter")
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestOpenAI`
Expected: FAIL — `undefined: NewOpenAI`

- [ ] **Step 3: Реализовать `openai.go`**

```go
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
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestOpenAI`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/openai.go internal/provider/openai_test.go
git commit -m "feat: add OpenAI key checker"
```

---

### Task 5: Провайдер Anthropic

**Files:**
- Create: `internal/provider/anthropic.go`
- Create: `internal/provider/anthropic_test.go`

**Interfaces:**
- Consumes: `Provider`, `Result`, `Mask`.
- Produces: `func NewAnthropic() *Anthropic`; `ID()=="anthropic"`; `Matches` истинен для `sk-ant-`. Использует заголовки `x-api-key` и `anthropic-version: 2023-06-01`. Баланс `nil`.

- [ ] **Step 1: Написать падающий тест `anthropic_test.go`**

```go
package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicValid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-ant-good" || r.Header.Get("anthropic-version") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-3-5-sonnet"}]}`))
	}))
	defer ts.Close()

	p := NewAnthropic()
	p.baseURL = ts.URL

	res, err := p.Check(context.Background(), "sk-ant-good")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Provider != "anthropic" {
		t.Fatalf("expected valid anthropic, got %+v", res)
	}
}

func TestAnthropicMatches(t *testing.T) {
	if !NewAnthropic().Matches("sk-ant-api03-x") {
		t.Fatal("should match sk-ant-")
	}
	if NewAnthropic().Matches("sk-proj-x") {
		t.Fatal("should not match plain sk-")
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestAnthropic`
Expected: FAIL — `undefined: NewAnthropic`

- [ ] **Step 3: Реализовать `anthropic.go`**

```go
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
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestAnthropic`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/anthropic.go internal/provider/anthropic_test.go
git commit -m "feat: add Anthropic key checker"
```

---

### Task 6: Провайдер OpenRouter (с балансом)

**Files:**
- Create: `internal/provider/openrouter.go`
- Create: `internal/provider/openrouter_test.go`

**Interfaces:**
- Consumes: `Provider`, `Result`, `Balance`, `Mask`.
- Produces: `func NewOpenRouter() *OpenRouter`; `ID()=="openrouter"`; `Matches` истинен для `sk-or-`. Запрашивает `GET /api/v1/key`, парсит `{"data":{"usage":..,"limit":..,"limit_remaining":..}}` в `Balance` (`Amount`=`limit_remaining` при наличии, иначе `usage`; `Limit`=`limit`; `Currency`="USD").

- [ ] **Step 1: Написать падающий тест `openrouter_test.go`**

```go
package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenRouterValidWithBalance(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-or-good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"label":"key","usage":3.5,"limit":10,"limit_remaining":6.5}}`))
	}))
	defer ts.Close()

	p := NewOpenRouter()
	p.baseURL = ts.URL

	res, err := p.Check(context.Background(), "sk-or-good")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Fatalf("expected valid, got %+v", res)
	}
	if res.Balance == nil || res.Balance.Amount != 6.5 {
		t.Fatalf("expected balance 6.5, got %+v", res.Balance)
	}
	if res.Balance.Limit == nil || *res.Balance.Limit != 10 {
		t.Fatalf("expected limit 10, got %+v", res.Balance)
	}
}

func TestOpenRouterMatches(t *testing.T) {
	if !NewOpenRouter().Matches("sk-or-v1-abc") {
		t.Fatal("should match sk-or-")
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestOpenRouter`
Expected: FAIL — `undefined: NewOpenRouter`

- [ ] **Step 3: Реализовать `openrouter.go`**

```go
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
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestOpenRouter`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/openrouter.go internal/provider/openrouter_test.go
git commit -m "feat: add OpenRouter key checker with balance"
```

---

### Task 7: Провайдер Perplexity

**Files:**
- Create: `internal/provider/perplexity.go`
- Create: `internal/provider/perplexity_test.go`

**Interfaces:**
- Consumes: `Provider`, `Result`, `Mask`.
- Produces: `func NewPerplexity() *Perplexity`; `ID()=="perplexity"`; `Matches` истинен для `pplx-`. Валидирует минимальным `POST /chat/completions` (`max_tokens:1`); 200 → валиден, 401/403 → невалиден. Баланс `nil`.

> Примечание для исполнителя: у Perplexity нет бесплатного эндпоинта только-для-проверки; минимальный chat-запрос может стоить копейки. Юнит-тест мокает ответ, поэтому логика парсинга проверяется без сети. Перед прод-использованием сверить актуальную документацию Perplexity; при появлении дешёвого GET-эндпоинта заменить тело `Check`, тест остаётся валидным по статус-кодам.

- [ ] **Step 1: Написать падающий тест `perplexity_test.go`**

```go
package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPerplexityValid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer pplx-good" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi"}}]}`))
	}))
	defer ts.Close()

	p := NewPerplexity()
	p.baseURL = ts.URL

	res, err := p.Check(context.Background(), "pplx-good")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Provider != "perplexity" {
		t.Fatalf("expected valid perplexity, got %+v", res)
	}
}

func TestPerplexityInvalid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	p := NewPerplexity()
	p.baseURL = ts.URL

	res, _ := p.Check(context.Background(), "pplx-bad")
	if res.Valid {
		t.Fatal("expected invalid")
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestPerplexity`
Expected: FAIL — `undefined: NewPerplexity`

- [ ] **Step 3: Реализовать `perplexity.go`**

```go
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
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestPerplexity`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/perplexity.go internal/provider/perplexity_test.go
git commit -m "feat: add Perplexity key checker"
```

---

### Task 8: Провайдер FAL.ai

**Files:**
- Create: `internal/provider/fal.go`
- Create: `internal/provider/fal_test.go`

**Interfaces:**
- Consumes: `Provider`, `Result`, `Mask`.
- Produces: `func NewFal() *Fal`; `ID()=="fal"`; `Matches` истинен для ключей формата `<id>:<secret>` (содержат ровно один `:` и не начинаются с известных префиксов) ИЛИ префикса `fal-`/`key-`. Аутентификация заголовком `Authorization: Key <key>`. Баланс `nil`.

> Примечание для исполнителя: FAL-ключи имеют формат `key_id:key_secret`. У FAL нет стабильного публичного «только проверить» эндпоинта; используем лёгкий GET к базовому REST-хосту и трактуем 401/403 как невалидный, 200/2xx как валидный. Юнит-тест мокает ответ. Перед прод-использованием сверить актуальную документацию FAL и при необходимости заменить URL в `Check` — тест по статус-кодам остаётся валидным.

- [ ] **Step 1: Написать падающий тест `fal_test.go`**

```go
package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFalValid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Key id:secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	p := NewFal()
	p.baseURL = ts.URL

	res, err := p.Check(context.Background(), "id:secret")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Provider != "fal" {
		t.Fatalf("expected valid fal, got %+v", res)
	}
}

func TestFalMatches(t *testing.T) {
	if !NewFal().Matches("abcd1234:efgh5678") {
		t.Fatal("should match id:secret form")
	}
	if NewFal().Matches("sk-ant-abc") {
		t.Fatal("should not match anthropic key")
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestFal`
Expected: FAIL — `undefined: NewFal`

- [ ] **Step 3: Реализовать `fal.go`**

```go
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
	// Формат key_id:key_secret — ровно один двоеточие, обе части непустые,
	// и это не ключ другого провайдера.
	if strings.HasPrefix(key, "sk-") || strings.HasPrefix(key, "pplx-") {
		return false
	}
	parts := strings.Split(key, ":")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

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
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/provider/ -run TestFal`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/provider/fal.go internal/provider/fal_test.go
git commit -m "feat: add FAL.ai key checker"
```

---

### Task 9: Реестр провайдеров и фан-аут-проверка

**Files:**
- Create: `internal/check/service.go`
- Create: `internal/check/service_test.go`
- Modify: `go.mod` (добавится зависимость errgroup через `go get`)

**Interfaces:**
- Consumes: `provider.Provider`, `provider.Detect`, `provider.Result`, `provider.Mask`, все `New*` конструкторы.
- Produces:
  - `func DefaultProviders() []provider.Provider` — все пять провайдеров в порядке: anthropic, openrouter, openai, perplexity, fal (специфичные префиксы раньше общего `sk-`).
  - `type Service struct { ... }` с `func NewService(ps []provider.Provider) *Service`.
  - `func (s *Service) CheckAll(ctx context.Context, keys []string) []provider.Result` — параллельно (лимит 8), сохраняет порядок ввода; неопознанный ключ → `Result{Key: Mask(key), Provider:"unknown", Valid:false, Detail:"провайдер не распознан"}`; ошибка `Check` → `Valid:false`, `Detail` с текстом ошибки (ключ в тексте не появляется).

- [ ] **Step 1: Добавить зависимость errgroup**

```bash
export PATH="$HOME/.local/go/bin:$PATH"
go get golang.org/x/sync/errgroup
```

- [ ] **Step 2: Написать падающий тест `service_test.go`**

```go
package check

import (
	"context"
	"testing"

	"github.com/kravtandr/keychecker/internal/provider"
)

type stubProvider struct {
	id, prefix string
	valid      bool
}

func (s stubProvider) ID() string            { return s.id }
func (s stubProvider) Matches(k string) bool { return len(k) >= len(s.prefix) && k[:len(s.prefix)] == s.prefix }
func (s stubProvider) Check(_ context.Context, k string) (provider.Result, error) {
	return provider.Result{Key: provider.Mask(k), Provider: s.id, Valid: s.valid}, nil
}

func TestCheckAllOrderAndUnknown(t *testing.T) {
	ps := []provider.Provider{
		stubProvider{"a", "aa-", true},
		stubProvider{"b", "bb-", false},
	}
	svc := NewService(ps)

	keys := []string{"aa-1111111111", "zz-9999999999", "bb-2222222222"}
	got := svc.CheckAll(context.Background(), keys)

	if len(got) != 3 {
		t.Fatalf("want 3 results, got %d", len(got))
	}
	if got[0].Provider != "a" || !got[0].Valid {
		t.Errorf("result 0 wrong: %+v", got[0])
	}
	if got[1].Provider != "unknown" || got[1].Valid {
		t.Errorf("result 1 should be unknown: %+v", got[1])
	}
	if got[2].Provider != "b" || got[2].Valid {
		t.Errorf("result 2 wrong: %+v", got[2])
	}
	for _, r := range got {
		if len(r.Key) > 0 && r.Key[:2] != "**" && len(r.Key) > 13 {
			t.Errorf("key not masked: %q", r.Key)
		}
	}
}
```

- [ ] **Step 3: Запустить тест — убедиться, что падает**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/check/`
Expected: FAIL — `undefined: NewService`

- [ ] **Step 4: Реализовать `service.go`**

```go
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
```

- [ ] **Step 5: Запустить тест — убедиться, что проходит**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/check/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/check/
git commit -m "feat: add provider registry and parallel check service"
```

---

### Task 10: Auth middleware (Bearer-токен, fail-closed)

**Files:**
- Create: `internal/httpapi/auth.go`
- Create: `internal/httpapi/auth_test.go`

**Interfaces:**
- Consumes: ничего.
- Produces: `func RequireToken(token string, next http.Handler) http.Handler` — сравнивает `Authorization: Bearer <token>` через `subtle.ConstantTimeCompare`; при несовпадении/отсутствии → 401 JSON `{"error":"unauthorized"}`. Пустой `token` считается ошибкой конфигурации: конструктор возвращает handler, всегда отдающий 500 (страховка; основной fail-closed — в `main`).

- [ ] **Step 1: Написать падающий тест `auth_test.go`**

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireToken(t *testing.T) {
	h := RequireToken("secret", okHandler())

	cases := []struct {
		name, header string
		want         int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"wrong", "Bearer nope", http.StatusUnauthorized},
		{"right", "Bearer secret", http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/check", nil)
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != c.want {
				t.Fatalf("got %d, want %d", rr.Code, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/httpapi/ -run TestRequireToken`
Expected: FAIL — `undefined: RequireToken`

- [ ] **Step 3: Реализовать `auth.go`**

```go
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
```

- [ ] **Step 4: Запустить тест — убедиться, что проходит**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/httpapi/ -run TestRequireToken`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi/auth.go internal/httpapi/auth_test.go
git commit -m "feat: add Bearer token auth middleware"
```

---

### Task 11: HTTP-хендлер /api/check и сборка сервера

**Files:**
- Create: `internal/httpapi/handler.go`
- Create: `internal/httpapi/handler_test.go`
- Create: `internal/httpapi/static.go`
- Modify: `cmd/keychecker/main.go`

**Interfaces:**
- Consumes: `check.Service`, `check.DefaultProviders`, `check.NewService`, `RequireToken`, `provider.Result`.
- Produces:
  - `type CheckRequest struct { Keys []string \`json:"keys"\` }`.
  - `type checker interface { CheckAll(ctx, keys []string) []provider.Result }`.
  - `func CheckHandler(svc checker) http.Handler` — `POST`, декодит JSON, режет пустые строки/пробелы, ограничивает 200 ключей, отвечает `{"results":[...]}`.
  - `func StaticHandler() http.Handler` — отдаёт встроенный `web/dist` (SPA fallback на `index.html`).
  - `func NewMux(token string, svc checker) http.Handler` — маршрутизация: `/healthz` открыт; `/api/check` под `RequireToken`; всё остальное — статика.

- [ ] **Step 1: Написать падающий тест `handler_test.go`**

```go
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
```

- [ ] **Step 2: Запустить тест — убедиться, что падает**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go test ./internal/httpapi/ -run TestCheckHandler`
Expected: FAIL — `undefined: CheckHandler`

- [ ] **Step 3: Реализовать `handler.go`**

```go
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
```

- [ ] **Step 4: Реализовать `static.go`**

```go
package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:webdist
var webDist embed.FS

// StaticHandler отдаёт собранный фронт с SPA-fallback на index.html.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(webDist, "webdist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(sub, trimSlash(r.URL.Path)); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

func trimSlash(p string) string {
	if len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	if p == "" {
		return "index.html"
	}
	return p
}
```

Создать плейсхолдер, чтобы `embed` компилировался до сборки фронта:

```bash
mkdir -p internal/httpapi/webdist
printf '<!doctype html><title>KeyChecker</title>placeholder' > internal/httpapi/webdist/index.html
```

- [ ] **Step 5: Реализовать `NewMux` в `handler.go` (дописать)**

```go
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
```

- [ ] **Step 6: Переписать `cmd/keychecker/main.go`**

```go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/kravtandr/keychecker/internal/check"
	"github.com/kravtandr/keychecker/internal/httpapi"
)

func main() {
	token := os.Getenv("KEYCHECKER_TOKEN")
	if token == "" {
		log.Fatal("KEYCHECKER_TOKEN is required (fail-closed): refusing to start without an auth token")
	}
	addr := os.Getenv("KEYCHECKER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	svc := check.NewService(check.DefaultProviders())
	mux := httpapi.NewMux(token, svc)

	log.Printf("keychecker listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
```

Удалить старый `newMux`/`main_test.go` из `cmd/keychecker` (health теперь в `NewMux`):

```bash
rm cmd/keychecker/main_test.go
```

- [ ] **Step 7: Запустить все тесты — убедиться, что проходят**

Run: `export PATH="$HOME/.local/go/bin:$PATH" && go build ./... && go test ./...`
Expected: PASS во всех пакетах

- [ ] **Step 8: Commit**

```bash
git add internal/httpapi/ cmd/keychecker/main.go
git rm --cached cmd/keychecker/main_test.go 2>/dev/null || true
git commit -m "feat: wire /api/check handler, static SPA serving and server bootstrap"
```

---

### Task 12: Фронтенд React/Vite

**Files:**
- Create: `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json`, `web/index.html`
- Create: `web/src/main.tsx`, `web/src/App.tsx`, `web/src/api.ts`, `web/src/styles.css`

**Interfaces:**
- Consumes: бэкенд `POST /api/check` с заголовком `Authorization: Bearer <token>`, тело `{keys: string[]}`, ответ `{results: Result[]}` где `Result = {key, provider, valid, balance?: {amount, currency, limit?}, detail}`.
- Produces: SPA, собираемая в `web/dist`.

- [ ] **Step 1: Скаффолд Vite-проекта**

```bash
cd /home/kravtandr/proj/KeyChecker
npm create vite@latest web -- --template react-ts
cd web && npm install
```

- [ ] **Step 2: Настроить `vite.config.ts` (dist + прокси на бэкенд в dev)**

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: { outDir: 'dist' },
  server: {
    proxy: { '/api': 'http://localhost:8080' },
  },
})
```

- [ ] **Step 3: Реализовать `web/src/api.ts`**

```ts
export type Balance = { amount: number; currency: string; limit?: number }
export type Result = {
  key: string
  provider: string
  valid: boolean
  balance?: Balance
  detail: string
}

export async function checkKeys(token: string, keys: string[]): Promise<Result[]> {
  const resp = await fetch('/api/check', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ keys }),
  })
  if (resp.status === 401) throw new Error('Неверный токен доступа')
  if (!resp.ok) throw new Error(`Ошибка сервера: ${resp.status}`)
  const data = await resp.json()
  return data.results as Result[]
}
```

- [ ] **Step 4: Реализовать `web/src/App.tsx`**

```tsx
import { useState } from 'react'
import { checkKeys, type Result } from './api'
import './styles.css'

export default function App() {
  const [token, setToken] = useState('')
  const [raw, setRaw] = useState('')
  const [results, setResults] = useState<Result[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function onCheck() {
    setError('')
    setResults([])
    const keys = raw.split('\n').map((s) => s.trim()).filter(Boolean)
    if (keys.length === 0) {
      setError('Введите хотя бы один ключ')
      return
    }
    setLoading(true)
    try {
      setResults(await checkKeys(token, keys))
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="wrap">
      <h1>KeyChecker</h1>
      <label>
        Токен доступа
        <input
          type="password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder="KEYCHECKER_TOKEN"
        />
      </label>
      <label>
        Ключи (по одному на строку)
        <textarea
          rows={8}
          value={raw}
          onChange={(e) => setRaw(e.target.value)}
          placeholder={'sk-...\nsk-ant-...\nsk-or-v1-...'}
        />
      </label>
      <button onClick={onCheck} disabled={loading}>
        {loading ? 'Проверяю…' : 'Проверить'}
      </button>
      {error && <p className="error">{error}</p>}
      {results.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>Ключ</th>
              <th>Провайдер</th>
              <th>Статус</th>
              <th>Баланс</th>
              <th>Детали</th>
            </tr>
          </thead>
          <tbody>
            {results.map((r, i) => (
              <tr key={i}>
                <td className="mono">{r.key}</td>
                <td>{r.provider}</td>
                <td className={r.valid ? 'ok' : 'bad'}>{r.valid ? '✓ валиден' : '✗ невалиден'}</td>
                <td>
                  {r.balance
                    ? `${r.balance.amount} ${r.balance.currency}` +
                      (r.balance.limit != null ? ` / ${r.balance.limit}` : '')
                    : '—'}
                </td>
                <td>{r.detail}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  )
}
```

- [ ] **Step 5: Реализовать `web/src/main.tsx` и `web/src/styles.css`**

`main.tsx`:
```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

`styles.css`:
```css
:root { color-scheme: light dark; font-family: system-ui, sans-serif; }
.wrap { max-width: 900px; margin: 2rem auto; padding: 0 1rem; }
label { display: block; margin: 1rem 0; }
input, textarea { display: block; width: 100%; padding: 0.5rem; margin-top: 0.25rem; box-sizing: border-box; }
button { padding: 0.6rem 1.2rem; font-size: 1rem; cursor: pointer; }
button:disabled { opacity: 0.6; cursor: default; }
table { width: 100%; border-collapse: collapse; margin-top: 1.5rem; }
th, td { text-align: left; padding: 0.5rem; border-bottom: 1px solid #8884; }
.mono { font-family: ui-monospace, monospace; }
.ok { color: #16a34a; } .bad { color: #dc2626; }
.error { color: #dc2626; }
```

Заменить содержимое `web/index.html` тела на `<div id="root"></div>` и скрипт `/src/main.tsx` (по умолчанию из шаблона уже так; убрать лишнее из шаблона).

- [ ] **Step 6: Собрать фронт и проверить, что билд проходит**

Run: `cd /home/kravtandr/proj/KeyChecker/web && npm run build`
Expected: каталог `web/dist` создан без ошибок

- [ ] **Step 7: Commit**

```bash
cd /home/kravtandr/proj/KeyChecker
git add web/ -- ':!web/node_modules' ':!web/dist'
git commit -m "feat: add React/Vite frontend for batch key checking"
```

---

### Task 13: Docker (multi-stage) и локальный прогон

**Files:**
- Create: `Dockerfile`
- Create: `.dockerignore`
- Modify: `internal/httpapi/static.go` не требует изменений — Dockerfile кладёт `web/dist` в `internal/httpapi/webdist` перед `go build`.

**Interfaces:**
- Consumes: собранный фронт, Go-исходники.
- Produces: образ `keychecker`, слушающий `:8080`, требующий `KEYCHECKER_TOKEN`.

- [ ] **Step 1: Создать `.dockerignore`**

```
web/node_modules
web/dist
internal/httpapi/webdist
.git
keychecker
```

- [ ] **Step 2: Создать `Dockerfile`**

```dockerfile
# --- frontend build ---
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- backend build ---
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./internal/httpapi/webdist
RUN CGO_ENABLED=0 go build -o /out/keychecker ./cmd/keychecker

# --- runtime ---
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/keychecker /keychecker
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/keychecker"]
```

- [ ] **Step 3: Собрать образ**

Run: `cd /home/kravtandr/proj/KeyChecker && docker build -t keychecker .`
Expected: успешная сборка, финальный образ создан

- [ ] **Step 4: Прогнать контейнер и проверить fail-closed + health**

```bash
# без токена — должен упасть
docker run --rm keychecker || echo "fail-closed OK"
# с токеном — health отвечает
docker run --rm -d -p 8080:8080 -e KEYCHECKER_TOKEN=testtoken --name kc keychecker
sleep 1
curl -s localhost:8080/healthz
curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/api/check   # ожидаем 401
docker rm -f kc
```
Expected: `fail-closed OK`, затем `ok`, затем `401`

- [ ] **Step 5: Commit**

```bash
git add Dockerfile .dockerignore
git commit -m "feat: add multi-stage Docker build"
```

---

### Task 14: Применение vibe-coding-guidelines и README

**Files:**
- Create: `AGENTS.md`
- Create: `CLAUDE.md`
- Create: `docs/adr/0001-adopt-vibe-coding-guidelines.md`
- Create: `.github/workflows/ci.yml`
- Create: `README.md`

**Interfaces:**
- Consumes: канонические команды проекта из предыдущих задач.
- Produces: установленные правила агентной разработки (следуя `AGENTS-GUIDE.md` из `github.com/kravtandr/vibe-coding-guidelines`), CI и README.

> Примечание для исполнителя: следуй `AGENTS-GUIDE.md` референса — разреши тег `v0.1.0` в конкретный commit SHA и запиши его в provenance-заголовок `AGENTS.md` и в ADR. Ниже — канонические команды проекта.

Канонические команды:
- LINT: `gofmt -l . && go vet ./...`
- FAST_TEST / TEST: `go test ./...`
- BUILD: `go build ./... && (cd web && npm ci && npm run build)`

- [ ] **Step 1: Создать `AGENTS.md`** с provenance-заголовком (source locator `https://github.com/kravtandr/vibe-coding-guidelines`, разрешённый SHA тега `v0.1.0`, использованные шаблоны) и секцией команд выше; правило безопасности: ключи никогда не логируются и не сохраняются.

- [ ] **Step 2: Создать `CLAUDE.md`** из одной строки-адаптера:

```
@AGENTS.md
```

- [ ] **Step 3: Создать ADR `docs/adr/0001-adopt-vibe-coding-guidelines.md`** по шаблону из референса: контекст, решение (принять гайдлайны на SHA), записанный source revision, гейты.

- [ ] **Step 4: Создать `.github/workflows/ci.yml`**

```yaml
name: CI
on:
  push: { branches: [main] }
  pull_request:
permissions: { contents: read }
jobs:
  build-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - uses: actions/setup-node@v4
        with: { node-version: '22' }
      - name: Lint
        run: gofmt -l . && go vet ./...
      - name: Build frontend (embed placeholder ok)
        run: cd web && npm ci && npm run build && mkdir -p ../internal/httpapi/webdist && cp -r dist/* ../internal/httpapi/webdist/
      - name: Test
        run: go test ./...
      - name: Build
        run: go build ./...
```

- [ ] **Step 5: Создать `README.md`** — назначение, `docker build`/`docker run` с `KEYCHECKER_TOKEN`, dev-режим (`go run` + `npm run dev`), таблица провайдеров и что баланс есть только у OpenRouter, предупреждение о безопасности ключей.

- [ ] **Step 6: Финальная проверка всех гейтов локально**

Run:
```bash
export PATH="$HOME/.local/go/bin:$PATH"
gofmt -l . && go vet ./... && go test ./... && go build ./...
```
Expected: чисто, все тесты PASS

- [ ] **Step 7: Commit**

```bash
git add AGENTS.md CLAUDE.md docs/adr/ .github/workflows/ci.yml README.md
git commit -m "chore: adopt vibe-coding-guidelines, add CI and README"
```

---

## Итоговая проверка покрытия спеки

- Провайдеры OpenAI/Anthropic/OpenRouter/Perplexity/FAL → Tasks 4–8.
- Автоопределение по префиксу → Task 3 + порядок в Task 9.
- Баланс где доступно (OpenRouter) → Task 6; пометка «недоступно» → Tasks 4,5,7,8.
- Ввод пачкой, порядок ввода → Tasks 9, 11, 12.
- Bearer-токен, fail-closed → Tasks 10, 11 (main).
- Маскирование, ключи не логируются/не хранятся → Task 2 + сквозная маскировка.
- SSRF (фиксированные хосты) → захардкоженные baseURL в Tasks 4–8.
- Один Docker-образ (Go + embed фронта) → Tasks 11, 13.
- Тесты на моках без реальной сети → Tasks 2–11.
- Применение гайдлайнов, CI → Task 14.
