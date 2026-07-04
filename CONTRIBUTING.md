# Contributing to Xiaomi VLESS Panel

Спасибо за интерес к проекту. Репозиторий — экспериментальная панель для **Xiaomi BE7000**; вклад приветствуется, но поддержка не гарантируется.

## Перед началом

1. Прочитайте [docs/overview.md](docs/overview.md) — ограничения и архитектура.
2. Убедитесь, что есть Go 1.22+ и `make`.
3. Для UI/iptables без роутера: [lab/README.md](lab/README.md) (`make lab-up`).

## Сборка и тесты

```bash
make tidy
make test
make build-arm64
```

Локальный запуск панели:

```bash
PANEL_CONFIG=./deploy/panel.json.example go run ./cmd/panel -listen 127.0.0.1:7777
```

Lab-среда (Multipass):

```bash
make lab-up
make lab-deploy      # после изменений в коде
make lab-guest-test
```

## Стиль коммитов

Используйте [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` — новая функциональность
- `fix:` — исправление бага
- `docs:` — только документация
- `refactor:` — рефакторинг без изменения поведения
- `test:` — тесты
- `chore:` — инфраструктура, зависимости

Пример: `feat(panel): show update banner on dashboard`

## Pull Request

1. Создайте ветку от `main`.
2. Добавьте или обновите тесты для изменений в Go-коде.
3. Запустите `make test` локально.
4. Обновите `CHANGELOG.md` (секция `[Unreleased]`) для заметных изменений.
5. Опишите PR: что сделано, как проверить, ограничения (если есть).

## Области кода

| Каталог | Назначение |
|---------|------------|
| `cmd/panel/` | Точка входа |
| `internal/server/` | HTTP, UI, auth |
| `internal/service/` | Бизнес-логика |
| `internal/xray/` | Генерация config, iptables |
| `internal/subscription/` | Парсинг подписок |
| `deploy/`, `scripts/` | Установка на роутер |
| `lab/` | VM для разработки |

## UI и i18n

Статические страницы: `internal/server/static/`. Строки — в `locales/ru.json` и `locales/en.json`. Новые ключи добавляйте в оба файла.

## Релизы

Релизы публикуются через GitHub Actions при push тега `v*.*.*`:

```bash
make test
make tag-release VERSION=v0.3.0
```

Подробнее: [docs/development.md](docs/development.md).

## Безопасность

Не коммитьте пароли, токены подписок и `panel.json` с реальными данными. Сообщения об уязвимостях — через Issues репозитория (без публикации эксплойтов).
