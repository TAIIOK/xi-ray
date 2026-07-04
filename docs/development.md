# Разработка

## Требования

- Go 1.22+ (см. `go.mod`)
- Make
- Для кросс-сборки под роутер: `GOOS=linux GOARCH=arm64`

## Сборка и тесты

```bash
make tidy
make test
make build-arm64
```

Бинарник: `dist/panel-linux-arm64`

## Локальный запуск

```bash
make build-arm64   # или go build ./cmd/panel
PANEL_CONFIG=./deploy/panel.json.example go run ./cmd/panel -listen 0.0.0.0:7777
```

Флаг `-listen` переопределяет `listen_addr` только на время запуска:

```bash
go run ./cmd/panel -config ./deploy/panel.json.example -listen 127.0.0.1:7777
```

Открыть: http://127.0.0.1:7777

На локальной машине `BootstrapOnStart` переносит пути с `/data/` и `/mnt/usb-*` в writable-каталог.

## Установка на роутер из dev-сборки

```bash
make build-arm64
ssh root@192.168.31.1 'sh -s' < deploy/install.sh
```

## Структура репозитория

```
cmd/panel/          — точка входа
internal/config/    — panel.json, paths, routing
internal/server/    — HTTP, static UI, auth
internal/service/   — бизнес-логика (apply, status, onboarding)
internal/xray/      — генерация config.json, iptables, API client
internal/subscription/ — парсинг подписок и vless://
internal/setup/     — bootstrap, скачивание Xray, embedded scripts
internal/update/    — self-update с GitHub Releases
deploy/             — install/uninstall, init.d, hotplug
scripts/            — shell-скрипты для роутера
```

## Релизы

```bash
make test
make release-bundle VERSION=v1.0.0   # локальная проверка bundle
make tag-release VERSION=v1.0.0      # tag → GitHub Actions → Release
```

GitHub Actions (`.github/workflows/release.yml`) при push tag `v*.*.*`:

- запускает тесты;
- собирает `xiaomi-vless-vX.Y.Z-linux-arm64.tar.gz`;
- публикует Release в [TAIIOK/xi-ray](https://github.com/TAIIOK/xi-ray).

## Self-update (архитектура)

1. Panel проверяет GitHub Releases API
2. Скачивает bundle с resume при обрыве
3. `panel-updater.sh` атомарно заменяет бинарник (`panel.previous` для rollback)
4. `panel.json` не трогается; состояние в `updates/state.json`

## CI

`.github/workflows/ci.yml` — тесты на push/PR.

## Тест обновления локально

```bash
scripts/test-update.sh
```
