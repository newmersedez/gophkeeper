# GophKeeper

[![Tests & Coverage](https://github.com/newmersedez/gophkeeper/actions/workflows/coverage.yml/badge.svg)](https://github.com/newmersedez/gophkeeper/actions/workflows/coverage.yml)
[![golangci-lint](https://github.com/newmersedez/gophkeeper/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/newmersedez/gophkeeper/actions/workflows/golangci-lint.yml)

Клиент-серверный менеджер паролей (финальный проект курса «Продвинутый Go-разработчик», Яндекс Практикум).

## Возможности

- Регистрация, аутентификация и авторизация (JWT Bearer)
- Хранение: credentials, текст, бинарные данные, банковские карты, OTP (TOTP)
- Клиентское шифрование (AES-256-GCM + PBKDF2) — сервер хранит только ciphertext
- Синхронизация между клиентами (JSON и бинарный gob-протокол)
- CLI для Windows / Linux / macOS с информацией о версии и дате сборки
- Простой TUI (`gophkeeper tui`)
- OpenAPI/Swagger: [`api/swagger.yaml`](api/swagger.yaml)

## Архитектура

```
cmd/server          — HTTP-сервер
cmd/client          — CLI-клиент
internal/domain     — модели
internal/crypto     — шифрование
internal/auth       — bcrypt + JWT
internal/otp        — TOTP
internal/protocol   — бинарный sync (gob)
internal/server/*   — storage, handlers, middleware, config
internal/client/*   — API-клиент, local store, CLI, TUI, app
api/swagger.yaml    — описание REST API
```

Хранилище сервера — **PostgreSQL** (pgx + golang-migrate).
Локальный кэш клиента — SQLite (`~/.gophkeeper/vault.db`).

## Быстрый старт

```bash
# зависимости
make deps

# PostgreSQL
createdb gophkeeper
export DATABASE_URI='postgres://localhost:5432/gophkeeper?sslmode=disable'
export TEST_DATABASE_URI='postgres://localhost:5432/gophkeeper_test?sslmode=disable'

# сервер (по умолчанию localhost:8080)
make run-server
# либо: ./bin/gophkeeper-server -d "$DATABASE_URI"

# клиент
make build-client
./bin/gophkeeper version
./bin/gophkeeper register -login alice -password secret
./bin/gophkeeper add text -title note -content "hello"
./bin/gophkeeper sync
./bin/gophkeeper list
./bin/gophkeeper tui
```

### Переменные окружения / флаги сервера

| Env / Flag | Описание | Default |
|---|---|---|
| `RUN_ADDRESS` / `-a` | адрес listen | `localhost:8080` |
| `DATABASE_URI` / `-d` | PostgreSQL DSN | **обязателен** |
| `JWT_SECRET` / `-j` | секрет JWT | dev-значение |

Клиент: `-server`, `-data`, либо `GOPHKEEPER_SERVER`.

## Сборка под разные ОС

```bash
make build-client-all
# артефакты в bin/: gophkeeper-linux-amd64, gophkeeper-darwin-arm64, gophkeeper-windows-amd64.exe
```

Версия и дата прошиваются через `-ldflags`.

## Тесты, покрытие и линтер

```bash
make test
make coverage
make coverage-check   # порог ≥75%
make coverage-html
make coverage-badge
make lint             # golangci-lint
```

CI (GitHub Actions): `coverage.yml`, `golangci-lint.yml`, `statictest.yml`.

## Безопасность

1. Пароль пользователя хешируется bcrypt на сервере.
2. Полезная нагрузка сейфа шифруется на клиенте master-паролем (тем же, что и пароль входа в упрощённой модели курса).
3. Транспорт — HTTPS в production (локально HTTP допустим для разработки).
4. Сессия клиента хранится в `~/.gophkeeper/vault.db`.
