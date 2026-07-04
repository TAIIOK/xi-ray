# Автозапуск

После перезагрузки роутера panel и Xray поднимаются автоматически, если USB подключён и onboarding завершён.

## Механизмы

| Механизм | Назначение |
|----------|------------|
| `/data/xiaomi-vless-boot.sh` | Основной скрипт на flash: ждёт USB (до 3 мин), затем panel + Xray |
| `/etc/hotplug.d/block/99-xiaomi-vless` | Старт при появлении USB (удобно, если флешку вставили после загрузки) |
| `/etc/init.d/xiaomi-vless-boot` | procd START=99, резервный запуск |
| `/data/startup_user.sh` | Hook Xiaomi (может не вызываться на всех прошивках) |
| `cron @reboot sleep 30` | Резервный запуск через 30 сек |
| `cron * * * * *` | Watchdog: поднять panel, если упал |
| `cron */2` | Пере-применение iptables каждые 2 мин |

> **Не используйте `uci firewall` include** для VPN-скриптов. Firewall ждёт их синхронно — при загрузке с USB роутер может **не выдать интернет** на основной LAN. Установщик удаляет `firewall.startup_xray_guest`, если он был.

## Типичная последовательность после reboot

1. Роутер загружается (1–3 мин до готовности LAN)
2. USB монтируется в `/mnt/usb-XXXXX/`
3. Запускается `/data/xiaomi-vless-boot.sh` (hotplug / cron / procd)
4. Скрипт ждёт появления `…/xiaomi-vless/panel`
5. Запускается panel на `0.0.0.0:7777`
6. Если есть `xray.env` — запускается `startup_xray_guest.sh`
7. Применяются iptables и sysctl (rp_filter)

Panel обычно доступен через **1–3 минуты** после reboot.

## Лог boot-скрипта

```bash
tail -f /data/xiaomi-vless-boot.log
```

Пример успешного завершения:

```
=== boot begin ===
USB home: /mnt/usb-ed49605f/xiaomi-vless
LAN check done
panel started pid …
xray startup launched
=== boot done ===
```

## Проверка после перезагрузки

```bash
tail -f /data/xiaomi-vless-boot.log
tail -f /mnt/usb-*/xiaomi-vless/xray-startup.log
pidof panel
pidof xray
iptables -t nat -L XRAY_GUEST_TCP -v -n | tail -3
```

## Ручная регистрация автозапуска

Только autostart, без переустановки panel:

```bash
ssh root@192.168.31.1 'INSTALL_DIR=/mnt/usb-XXX/xiaomi-vless USB_MOUNT=/mnt/usb-XXX sh -s' < deploy/install-autostart.sh
```

## Одноразовый фикс на живом роутере

```bash
ssh root@192.168.31.1 'sh -s' < deploy/fix-autostart.sh
```

## Рекомендуемый порядок загрузки с USB

Если при **холодной загрузке с флешкой** роутер ведёт себя нестабильно:

1. Загрузите роутер **без USB**
2. Дождитесь рабочего интернета на основной сети
3. Вставьте USB — сработает hotplug и поднимется panel

Подробнее: [Устранение неполадок](troubleshooting.md#ошибка-загрузки-роутера-с-usb-флешкой)
