# Xiaomi VLESS Panel

Лёгкая веб-панель для **Xiaomi BE7000**: управление Xray/VLESS, подписками и failover VPN **только для гостевой Wi‑Fi**. Основная домашняя сеть остаётся без прокси.

> **Статус проекта:** это **не готовое продуктовое решение**, а экспериментальный репозиторий **для ознакомления и самостоятельной доработки**. Ожидайте ручной настройки, зависимости от USB и особенностей прошивки Xiaomi. Используйте на свой риск.

**Полная документация:** [docs/](docs/README.md)

---

## Что делает сервис


| Компонент                | Назначение                                                                           |
| ------------------------ | ------------------------------------------------------------------------------------ |
| **Веб-панель** (`:7777`) | Dashboard, подписки, выбор серверов, настройки, логи, обновления                     |
| **Xray**                 | VLESS-клиент с прозрачным прокси (REDIRECT TCP + TProxy UDP)                         |
| **iptables**             | Перенаправление трафика **только** гостевой подсети (`192.168.33.0/24` по умолчанию) |
| **Автозапуск**           | Скрипты на flash роутера + hotplug USB + cron/procd                                  |


```
Browser (LAN) → Go panel (:7777) → panel.json
                              → config.json → Xray
                              → startup_xray_guest.sh → iptables
```

**Возможности панели:** мониторинг Xray и exit IP, импорт подписок и `vless://`, single/multi режим с failover (Xray Observatory), генерация `config.json`, `xray -test`, self-update из GitHub Releases.

---



## Подготовка (кратко)

Перед установкой нужно:

1. **Xiaomi BE7000** с root/SSH (`ssh root@192.168.31.1`)
2. **USB-накопитель**, смонтированный в `/mnt/usb-`* — panel и Xray хранятся на флешке
3. **Гостевая Wi‑Fi** включена; известна подсеть гостей (часто `192.168.33.0/24`)
4. **Подписка VPN** или ссылка `vless://…`
5. **Интернет на роутере** — для скачивания Xray и обновления подписок

Подробный чеклист: [docs/prerequisites.md](docs/prerequisites.md)

---



## Быстрый старт



### Установка из Release (роутер)

```bash
curl -fsSL https://raw.githubusercontent.com/TAIIOK/xi-ray/main/scripts/quick-install.sh | sh
```

После установки: **[http://192.168.31.1:7777/onboarding](http://192.168.31.1:7777/onboarding)** (`admin` / `admin`).

### Установка из исходников

```bash
make build-arm64
ssh root@192.168.31.1 'sh -s' < deploy/install.sh
```



### Первичная настройка (onboarding)

1. Сменить пароль по умолчанию
2. Выбрать USB, при необходимости скачать Xray
3. Импортировать подписку, выбрать сервер(ы)
4. **Завершить настройку** (Apply + restart Xray)

Подробнее: [docs/installation.md](docs/installation.md), [docs/usage.md](docs/usage.md)

---



## Ошибка загрузки роутера с USB-флешкой

На BE7000 при **холодной загрузке с уже вставленной флешкой** роутер иногда **не поднимает интернет** на основной Wi‑Fi (долгая загрузка, нет WAN/NAT). Без флешки всё работает нормально.

**Как исправить:**

1. Перезагрузите роутер **без USB-флешки**
2. Дождитесь полной загрузки и рабочего интернета
3. **Вставьте флешку** — hotplug запустит panel и Xray автоматически

Для постоянной эксплуатации удобнее **всегда загружать роутер без флешки**, затем подключать USB вручную.

> Не используйте `uci firewall` include для VPN-скриптов — это блокирует поднятие WAN при boot. Текущий установщик такой hook не создаёт и удаляет старый, если был.

Подробнее: [docs/troubleshooting.md](docs/troubleshooting.md)

---



## Автозапуск после reboot

Panel и Xray поднимаются через `/data/xiaomi-vless-boot.sh` (ждёт USB до 3 мин). Обычно panel доступен через **1–3 минуты** после перезагрузки.

```bash
tail -f /data/xiaomi-vless-boot.log
pidof panel xray
```

Подробнее: [docs/autostart.md](docs/autostart.md)

---



## Деинсталляция

```bash
sh deploy/uninstall.sh          # dry-run
sh deploy/uninstall.sh --yes    # полная очистка
```

Флаги: `--router-only`, `--keep-xray`. См. [docs/installation.md](docs/installation.md).

---



## Сборка

```bash
make tidy && make test && make build-arm64
```

Локально:

```bash
PANEL_CONFIG=./deploy/panel.json.example go run ./cmd/panel -listen 0.0.0.0:7777
```

Подробнее: [docs/development.md](docs/development.md)

---



## Документация


| Документ                                                             | Содержание                      |
| -------------------------------------------------------------------- | ------------------------------- |
| [docs/README.md](docs/README.md)                                     | Оглавление                      |
| [docs/overview.md](docs/overview.md)                                 | Обзор, ограничения, архитектура |
| [docs/prerequisites.md](docs/prerequisites.md)                       | Подготовка роутера              |
| [docs/installation.md](docs/installation.md)                         | Установка и удаление            |
| [docs/usage.md](docs/usage.md)                                       | Работа с панелью                |
| [docs/autostart.md](docs/autostart.md)                               | Автозапуск                      |
| [docs/configuration.md](docs/configuration.md)                       | panel.json и API                |
| [docs/troubleshooting.md](docs/troubleshooting.md)                   | Устранение неполадок            |
| [docs/development.md](docs/development.md)                           | Сборка и релизы                 |



---

