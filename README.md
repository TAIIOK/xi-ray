# Xiaomi VLESS Panel

Лёгкая веб-панель для **Xiaomi BE7000**: мониторинг Xray/VLESS, подписки, выбор серверов и failover для гостевой Wi‑Fi.

## Возможности

- Dashboard: статус Xray, exit IP, iptables counters
- Подписки: добавление URL, обновление, парсинг `vless://`
- Серверы: ручной импорт, single/multi режим, failover через Xray Observatory
- Apply: генерация `config.json`, `xray -test`, restart через startup script
- Настройки путей, пароля, observatory probe

## Архитектура

```
Browser (LAN) → Go panel (:7777) → panel.json
                              → config.json → Xray
                              → startup_xray_guest.sh → iptables
```

Гостевая сеть `192.168.33.0/24` проксируется через Xray. Основная LAN без VPN.

## Сборка

```bash
make tidy
make test
make build-arm64
```

Бинарник: `dist/panel-linux-arm64`

### Установка с GitHub Release (роутер)

**Вариант 1 — одна команда** (скачать последний релиз и установить):

```bash
curl -fsSL https://raw.githubusercontent.com/TAIIOK/xi-ray/main/scripts/quick-install.sh | sh
```

**Вариант 2 — вручную из архива:**

```bash
cd /tmp
wget https://github.com/TAIIOK/xi-ray/releases/latest/download/xiaomi-vless-vX.Y.Z-linux-arm64.tar.gz
tar xzf xiaomi-vless-vX.Y.Z-linux-arm64.tar.gz
sh install.sh
```

Скрипт `install.sh` из релиза:
- находит USB (`/mnt/usb-*`)
- копирует panel, scripts, updater на USB
- создаёт `panel.json` (если ещё нет)
- регистрирует autostart (procd, cron, startup_user.sh)
- **сразу запускает panel**

После установки откройте **http://192.168.31.1:7777/onboarding** (`admin` / `admin`).

### Установка из исходников (разработчик)

```bash
make build-arm64
ssh root@192.168.31.1 'sh -s' < deploy/install.sh
```

Или из репозитория на роутере:

```bash
make install
```

`deploy/install.sh` после копирования файлов:
- регистрирует autostart (procd, cron, `startup_user.sh`)
- **сразу запускает panel** (procd с respawn, иначе nohup)
- **запускает Xray**, если уже настроен (`xray.env`), иначе ждёт onboarding
- проверяет, что панель отвечает на `:7777`

Панель: **http://192.168.31.1:7777**

Логин по умолчанию: `admin` / `admin` (смените в настройках).

## Деинсталляция

Скрипт [`deploy/uninstall.sh`](deploy/uninstall.sh) полностью удаляет panel, автозапуск, iptables-правила и данные на USB. Без `--yes` — только просмотр (dry-run).

```bash
# Посмотреть, что будет удалено
sh deploy/uninstall.sh

# Полная очистка (роутер + USB xiaomi-vless + xray)
sh deploy/uninstall.sh --yes

# Только роутер (USB оставить)
sh deploy/uninstall.sh --yes --router-only

# USB: panel, но Xray оставить
sh deploy/uninstall.sh --yes --keep-xray
```

С роутера через SSH:

```bash
ssh root@192.168.31.1 'sh -s' < deploy/uninstall.sh -- --yes
```

Из релизного архива (рядом с `install.sh`):

```bash
sh uninstall.sh --yes
```

Что удаляется с роутера: `/data/xiaomi-vless-boot.sh`, hotplug, хуки в `/data/startup_user.sh`, cron, `uci firewall`, init.d-сервисы (`xiaomi-vless-boot`, panel, xray), `/etc/sysctl.d/99-xray-guest.conf`, iptables-цепочки `XRAY_GUEST_*`. Что удаляется с USB: каталоги `xiaomi-vless/` и `xray/` (если не указан `--keep-xray`). После деинсталляции рекомендуется перезагрузка для сброса sysctl.

## Автозапуск VPN после перезагрузки

`deploy/install.sh` регистрирует автозапуск Xray + iptables несколькими способами:

| Механизм | Назначение |
|----------|------------|
| `/data/xiaomi-vless-boot.sh` | ждёт USB (до 3 мин), затем panel + xray |
| `/etc/hotplug.d/block/99-xiaomi-vless` | старт при подключении USB (boot с флешкой) |
| `/etc/init.d/xiaomi-vless-boot` | procd START=99, резервный запуск |
| `/data/startup_user.sh` | hook Xiaomi (может не вызываться на всех прошивках) |
| `cron @reboot sleep 30` | резервный запуск через 30 сек |
| `cron * * * * *` | watchdog: поднять panel если упал |
| `cron */2` | пере-применение iptables каждые 2 мин |

> **Не используйте `uci firewall` include** для VPN-скриптов — firewall ждёт их синхронно, и при boot с USB роутер может не отдать интернет.

После reboot panel обычно доступен через **1–3 минуты**. Лог: `tail -f /data/xiaomi-vless-boot.log`

Скрипт `/data/xiaomi-vless-boot.sh` (копируется на flash при install):

- ждёт mount USB с panel/xray (до 3 мин)
- ждёт LAN
- запускает panel на `0.0.0.0:7777`
- запускает Xray через `startup_xray_guest.sh`

Скрипт `/mnt/usb-…/xiaomi-vless/startup_xray_guest.sh`:
- ждёт сеть
- применяет `sysctl` (rp_filter)
- запускает Xray
- применяет iptables с retry

Только автозапуск (без переустановки панели):

```bash
ssh root@192.168.31.1 'INSTALL_DIR=/mnt/usb-XXX/xiaomi-vless USB_MOUNT=/mnt/usb-XXX sh -s' < deploy/install-autostart.sh
```

Или одноразовый фикс на живом роутере:

```bash
ssh root@192.168.31.1 'sh -s' < deploy/fix-autostart.sh
```

Проверка после reboot:

```bash
tail -f /data/xiaomi-vless-boot.log
tail -f /mnt/usb-*/xiaomi-vless/xray-startup.log
pidof panel
pidof xray
iptables -t nat -L XRAY_GUEST_TCP -v -n | tail -3
```

## Конфигурация

Файл `/data/xiaomi-vless/panel.json`:

| Поле | Значение по умолчанию |
|------|----------------------|
| xray_bin | `/mnt/usb-ed49605f/xray/bin/xray` |
| xray_config | `/mnt/usb-ed49605f/xray/config.json` |
| listen_addr | `192.168.31.1:7777` |
| xray_api_addr | `127.0.0.1:10085` |
| guest_subnet | `192.168.33.0/24` |

**Apply** перегенерирует `config.json` и `/data/xray-guest-iptables.sh` под выбранные серверы.

## API

| Method | Path | Описание |
|--------|------|----------|
| GET | `/api/status` | Статус Xray и VPN |
| GET | `/api/nodes` | Список серверов |
| POST | `/api/subscriptions` | Добавить подписку |
| PUT | `/api/selection` | Выбор серверов |
| POST | `/api/apply` | Применить config + restart |
| POST | `/api/restart` | Restart Xray |
| GET | `/api/update/status` | Версия panel и статус обновления |
| GET | `/api/update/check` | Проверить GitHub Releases |
| POST | `/api/update/download` | Скачать bundle |
| POST | `/api/update/apply` | Установить обновление |
| POST | `/api/update/rollback` | Откат на panel.previous |

## Multi-server failover

1. На странице **Серверы** выберите 2+ nodes
2. Режим **Несколько + failover**
3. **Apply + Restart**

Xray Observatory проверяет outbounds и balancer переключает на живой сервер.

## Локальная разработка

```bash
make build-arm64   # или go build ./cmd/panel
PANEL_CONFIG=./deploy/panel.json.example go run ./cmd/panel -listen 0.0.0.0:7777
```

Флаг `-listen` (или env `PANEL_LISTEN`) переопределяет `listen_addr` только на время запуска, `panel.json` не меняется:

```bash
# все интерфейсы, порт из panel.json
go run ./cmd/panel -config ./deploy/panel.json.example -listen 0.0.0.0

# явный host:port
go run ./cmd/panel -listen 127.0.0.1:7777
```

Открыть: http://127.0.0.1:7777

## Релизы и обновление

### Публикация релиза (разработчик)

```bash
make test
make release-bundle VERSION=v1.0.0   # локальная проверка bundle
make tag-release VERSION=v1.0.0      # tag → GitHub Actions создаст Release
```

GitHub Actions (`.github/workflows/release.yml`) при push tag `v*.*.*`:
- запускает тесты
- собирает `xiaomi-vless-vX.Y.Z-linux-arm64.tar.gz` (panel + scripts + deploy + manifest.json)
- публикует Release в [TAIIOK/xi-ray](https://github.com/TAIIOK/xi-ray)

### Обновление из панели (роутер)

1. **Настройки → Обновление panel → Проверить обновления**
2. **Скачать** — загрузка с возобновлением при обрыве сети
3. **Обновить** — атомарная установка через `panel-updater.sh`
4. При ошибке — **Откатить** на `panel.previous`

Безопасность:
- `panel.json` не перезаписывается; перед apply создаётся backup
- бинарник заменяется через rename (running process не ломается)
- состояние в `updates/state.json` — boot resume после обрыва SSH/питания
- post-update health check (`xray -test`, panel binary) с auto-rollback

Версия panel: `./panel -version` или footer на Dashboard.

## Документация

- [docs/chat-vless-xiaomi-be7000.md](docs/chat-vless-xiaomi-be7000.md) — история настройки роутера
