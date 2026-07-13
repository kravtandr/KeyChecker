<!-- provenance
source: https://github.com/kravtandr/vibe-coding-guidelines
revision: 2ff8d73878e839a5eafe65369cf8f29fd326a204
reference-release: ad hoc source (main branch resolved to commit; no v0.1.0 tag published)
templates: AGENTS.md, DEPLOYMENT.md, adr/0000-template.md
setup-date: 2026-07-13
This header records where these rules came from. It does not grant any
future setup run permission to overwrite this file.
-->

# Agent rules for KeyChecker

These rules are mandatory for any coding agent working in this
repository. Values below were resolved at setup; a literal unresolved
double-curly placeholder token anywhere in this installed file is a
defect.

## Scope

KeyChecker — self-hosted веб-сервис (Go-бэкенд + встроенный React/Vite
фронт), проверяющий API-ключи LLM-провайдеров на валидность и баланс.
Эти правила действуют на весь репозиторий.

## Commands and gates

Run commands exactly as written. Each is non-interactive, reproducible
from a clean checkout, and returns a non-zero exit status on failure.

| Gate | Configuration | Command or control | Scope | Rationale |
| --- | --- | --- | --- | --- |
| Setup | REQUIRED | `go mod download && (cd web && npm ci)` | clean checkout | Go-модули и npm-зависимости фронта. |
| Verification procedure | REQUIRED | `gofmt -l . && go vet ./... && go test ./... && go build ./...` | whole project | Единый агрегат: формат, статанализ, тесты, сборка бэкенда. |
| Lint/static analysis | REQUIRED | `gofmt -l . && go vet ./...` | whole project | Штатные инструменты Go; `gofmt -l` печатает неотформатированные файлы. |
| Fast tests | REQUIRED | `go test ./...` | whole project | Все тесты на моках, укладываются в секунды — отдельный fast-scope не нужен. |
| Full tests | REQUIRED | `go test ./...` | whole project | Тот же набор; реальная сеть не задействуется. |
| Build/typecheck/package | REQUIRED | `go build ./... && (cd web && npm run build)` | whole project | Сборка Go-бинарника и продакшн-бандла фронта (tsc + vite). |
| Pre-commit | N/A | — | — | Хук-менеджер в проекте не настроен; гейты обеспечиваются CI. |
| CI | REQUIRED | `.github/workflows/ci.yml` | full required scope | GitHub Actions прогоняет lint, test, build. |
| Merge protection | BLOCKED | branch protection в настройках репозитория | protected branches | Требует действий владельца; из этой сессии настройки не проверить. |

This table stores configuration, not results.

## Behavior rules

**Безопасность ключей (проектное правило).** API-ключи, которые вводит
пользователь, НИКОГДА не логируются, не пишутся на диск и не кэшируются.
Во всех результатах, ошибках и логах ключ маскируется функцией
`provider.Mask`. Исходящие запросы идут только на захардкоженные хосты
провайдеров.

**Preservation.** Do not overwrite or revert unrelated user changes. Do
not replace an existing mechanism because creating a parallel one is
easier. Before deleting, disabling, or replacing existing project
configuration, obtain explicit approval.

**Tests.** New or changed executable behavior MUST have appropriate
automated tests unless the test gate above is explicitly `N/A`. A bug fix
SHOULD include a regression test that fails without the fix.

**Verification before done.** Before reporting work as complete, run the
verification procedure above and every `REQUIRED` gate affected by the
change. Report the exact command, exit status, and concise relevant
output. Never claim remote CI passed without observing the actual run for
the relevant revision. If any required result is `FAIL` or `NOT_RUN`, do
not use the words "done", "complete", "fixed", or "passing".

**Baseline failures.** A failure that predates your change stays visible:
report it as `FAIL (pre-existing)`. Never weaken or skip a failing gate,
add an unexplained exclusion, or bypass hooks with `--no-verify`.

**Honesty about facts.** Never invent commands, environment names, URLs,
credentials, or repository settings. Ask when a required project fact is
unknown. Never write secret values into files, logs, ADRs, or reports.

## Architecture decision records

ADR directory: `docs/adr/`

Write an ADR before or together with: a new dependency that establishes
project-wide or architectural policy; a new framework, database,
protocol, CI provider, or hook manager; an architecture or
module-boundary change; a compatibility-breaking public-interface change;
any other decision that is expensive or risky to reverse. Use the next
free identifier in the ADR directory.

## Pre-commit and CI

Pre-commit hooks give fast local feedback; CI verifies a revision in a
clean environment; only branch protection or merge rules provide actual
merge enforcement. These are not equivalent. CI runs the same canonical
commands recorded in the table above.

## Deployment

Deployment status: N/A

Есть `DEPLOYMENT.md`. Продакшн-деплой не настроен: сервис поставляется
как Docker-образ, разворачиваемый командой самостоятельно. Setup и
рутинная работа никогда не деплоят.

## Monorepo and component scope

Rules in this file apply repository-wide. Single project; no
component-specific rules.
