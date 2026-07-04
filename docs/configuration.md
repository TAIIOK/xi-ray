# Конфигурация

## panel.json

Основной файл конфигурации: `…/xiaomi-vless/panel.json` на USB (путь задаётся при установке).

### Пути (`paths`)

| Поле | Пример | Описание |
|------|--------|----------|
| `xray_bin` | `/mnt/usb-…/xray/bin/xray` | Бинарник Xray |
| `xray_config` | `/mnt/usb-…/xray/config.json` | Конфиг Xray (генерируется panel) |
| `startup_script` | `…/startup_xray_guest.sh` | Запуск Xray + iptables |
| `iptables_script` | `…/xray-guest-iptables.sh` | Правила для гостей |
| `panel_data_dir` | `…/xiaomi-vless` | Каталог данных panel |

### Сеть (`network`)

| Поле | По умолчанию | Описание |
|------|--------------|----------|
| `guest_subnet` | `192.168.33.0/24` | Подсеть гостевой Wi‑Fi |
| `listen_addr` | `192.168.31.1:7777` | Адрес веб-панели |
| `xray_api_addr` | `127.0.0.1:10085` | Xray Stats API |

### iptables (`iptables`)

| Поле | По умолчанию |
|------|--------------|
| `tcp_port` | `12346` (REDIRECT) |
| `udp_port` | `12345` (TProxy) |
| `socks_port` | `10808` |
| `api_port` | `10085` |

### Observatory (`observatory`)

Используется при failover:

```json
{
  "enabled": true,
  "probe_url": "https://www.google.com/generate_204",
  "probe_interval": "30s"
}
```

### Подписки (`subscriptions_policy`)

- `auto_refresh_enabled` — периодическое обновление подписок
- `auto_refresh_interval_min` — интервал в минутах
- `reselect_strategy` — стратегия перевыбора (`best_ping`)
- `auto_apply_on_change` — Apply при изменении nodes

### Пример

См. [`deploy/panel.json.example`](../deploy/panel.json.example).

## Apply

**Apply** перегенерирует:

- `config.json` Xray (inbounds, outbounds, routing, balancer)
- скрипт iptables для гостевой подсети

и выполняет `xray -test` перед restart.

## HTTP API

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
| POST | `/api/update/rollback` | Откат на `panel.previous` |

API требует авторизации (session cookie после login).

## Переменные окружения и флаги

| Параметр | Описание |
|----------|----------|
| `-config` | Путь к `panel.json` |
| `-listen` | Переопределить `listen_addr` на время запуска |
| `PANEL_CONFIG` | env-аналог `-config` |
| `PANEL_LISTEN` | env-аналог `-listen` |

Локальный запуск:

```bash
PANEL_CONFIG=./deploy/panel.json.example go run ./cmd/panel -listen 0.0.0.0:7777
```

## Файлы на USB после onboarding

```
/mnt/usb-XXXXX/
├── xiaomi-vless/
│   ├── panel                 # бинарник
│   ├── panel.json            # конфиг panel
│   ├── panel.log
│   ├── startup_xray_guest.sh
│   ├── xray-guest-iptables.sh
│   ├── xray-guest-sysctl.sh
│   ├── boot-xiaomi-vless.sh
│   ├── panel-updater.sh
│   ├── xray.env              # создаётся после onboarding
│   └── updates/              # self-update
└── xray/
    ├── bin/xray
    ├── config.json           # генерируется panel
    ├── geoip.dat
    └── geosite.dat
```

## Файлы на flash роутера

| Путь | Назначение |
|------|------------|
| `/data/xiaomi-vless-boot.sh` | Boot-скрипт |
| `/data/xiaomi-vless-boot.log` | Лог boot |
| `/etc/hotplug.d/block/99-xiaomi-vless` | Hotplug USB |
| `/etc/init.d/xiaomi-vless-boot` | procd service |
| `/etc/sysctl.d/99-xray-guest.conf` | rp_filter для TProxy |
