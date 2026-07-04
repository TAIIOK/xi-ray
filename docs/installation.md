# Установка

## Установка из GitHub Release (рекомендуется)

### Одной командой

На роутере по SSH:

```bash
curl -fsSL https://raw.githubusercontent.com/TAIIOK/xi-ray/main/scripts/quick-install.sh | sh
```

### Вручную из архива

```bash
cd /tmp
wget https://github.com/TAIIOK/xi-ray/releases/latest/download/xiaomi-vless-vX.Y.Z-linux-arm64.tar.gz
tar xzf xiaomi-vless-vX.Y.Z-linux-arm64.tar.gz
sh install.sh
```

### Что делает `install.sh`

1. Находит смонтированный USB (`/mnt/usb-*`)
2. Копирует `panel`, скрипты и updater в `…/xiaomi-vless/`
3. Создаёт `panel.json`, если его ещё нет
4. Регистрирует автозапуск (procd, cron, hotplug, `startup_user.sh`)
5. **Сразу запускает panel**

После установки откройте **http://192.168.31.1:7777/onboarding** (`admin` / `admin`).

## Установка из исходников (разработчик)

На машине разработчика:

```bash
make build-arm64
ssh root@192.168.31.1 'sh -s' < deploy/install.sh
```

Или на роутере из клонированного репозитория:

```bash
make install
```

`deploy/install.sh` дополнительно:

- запускает Xray, если уже настроен (`xray.env`);
- ждёт ответ panel на `:7777`.

## Первый запуск

1. Откройте `http://192.168.31.1:7777/onboarding`
2. Смените пароль по умолчанию
3. Укажите пути USB, при необходимости скачайте Xray из панели
4. Импортируйте подписку, выберите сервер(ы)
5. Нажмите **Завершить настройку** (Apply + restart Xray)

Подробнее: [Использование панели → Onboarding](usage.md#onboarding)

## Деинсталляция

Скрипт [`deploy/uninstall.sh`](../deploy/uninstall.sh) удаляет panel, автозапуск, iptables и данные на USB.

```bash
# Посмотреть, что будет удалено (dry-run)
sh deploy/uninstall.sh

# Полная очистка (роутер + USB)
sh deploy/uninstall.sh --yes

# Только роутер (USB оставить)
sh deploy/uninstall.sh --yes --router-only

# USB: panel, но Xray оставить
sh deploy/uninstall.sh --yes --keep-xray
```

С компьютера через SSH:

```bash
ssh root@192.168.31.1 'sh -s' < deploy/uninstall.sh -- --yes
```

**Удаляется с роутера:** `/data/xiaomi-vless-boot.sh`, hotplug, хуки в `/data/startup_user.sh`, cron, init.d-сервисы, `/etc/sysctl.d/99-xray-guest.conf`, цепочки `XRAY_GUEST_*`.

**Удаляется с USB:** каталоги `xiaomi-vless/` и `xray/` (если не `--keep-xray`).

После деинсталляции рекомендуется **перезагрузка** роутера.

## Обновление panel

Из панели: **Настройки → Обновление panel**.

1. Проверить обновления
2. Скачать
3. Обновить
4. При ошибке — **Откатить** на `panel.previous`

`panel.json` не перезаписывается. Подробности — в [README](../README.md#релизы-и-обновление) и [Разработка](development.md).
