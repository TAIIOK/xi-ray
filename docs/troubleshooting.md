# Устранение неполадок

## Ошибка загрузки роутера с USB-флешкой

### Симптомы

- Роутер **не выдаёт интернет** на основной Wi‑Fi после включения **с уже вставленной** USB-флешкой
- Долгая загрузка, «зависание» сети, нет доступа к `192.168.31.1`
- WAN/NAT не поднимается, хотя без флешки всё работает

### Причина

На прошивке Xiaomi BE7000 загрузка с USB иногда конфликтует с порядком инициализации сети и firewall. Раньше проблему усугублял hook через `uci firewall include` — скрипты VPN выполнялись **синхронно** при старте firewall и блокировали поднятие WAN.

Текущий установщик **не использует** `uci firewall` для VPN и удаляет старый `firewall.startup_xray_guest`, но холодная загрузка с флешкой всё равно может быть нестабильной на отдельных прошивках.

### Решение

**Перезагрузите роутер без USB-флешки:**

1. Выключите роутер, **извлеките USB**
2. Включите роутер, дождитесь полной загрузки и рабочего интернета
3. **Вставьте USB** — hotplug запустит `/data/xiaomi-vless-boot.sh`
4. Через 1–2 минуты проверьте panel: `http://192.168.31.1:7777`

Альтернатива для постоянной эксплуатации: **всегда загружайте роутер без флешки**, затем подключайте USB вручную.

```bash
# После вставки USB — проверка
tail -f /data/xiaomi-vless-boot.log
pidof panel
```

---

## Panel не открывается после reboot

1. Проверьте USB: `ls /mnt/usb-*/xiaomi-vless/panel`
2. Лог boot: `tail -50 /data/xiaomi-vless-boot.log`
3. Лог panel: `tail -50 /mnt/usb-*/xiaomi-vless/panel.log`
4. Процесс: `pidof panel`
5. Ручной запуск:

```bash
/mnt/usb-XXXXX/xiaomi-vless/panel -config /mnt/usb-XXXXX/xiaomi-vless/panel.json -listen 0.0.0.0:7777
```

6. Переустановите autostart: `deploy/fix-autostart.sh`

---

## Xray не запускается

| Проверка | Команда |
|----------|---------|
| Конфиг существует | `ls /mnt/usb-*/xray/config.json` |
| Тест конфига | `/mnt/usb-*/xray/bin/xray run -test -c …/config.json` |
| Onboarding завершён | `xray.env` в каталоге panel |
| Лог startup | `tail -50 /mnt/usb-*/xiaomi-vless/xray-startup.log` |
| Процесс | `pidof xray` |

Ручной перезапуск:

```bash
sh /mnt/usb-XXXXX/xiaomi-vless/startup_xray_guest.sh
```

---

## VPN не работает на гостевой Wi‑Fi

1. **Подсеть гостей** в `panel.json` совпадает с реальной (`guest_subnet`)
2. Устройство подключено именно к **гостевой** сети
3. iptables применены:

```bash
iptables -t nat -L XRAY_GUEST_TCP -v -n
iptables -t mangle -L XRAY_GUEST_UDP -v -n
```

4. Счётчики растут при трафике гостя
5. SOCKS на роутере работает:

```bash
curl -x socks5h://127.0.0.1:10808 https://ifconfig.me
```

Если SOCKS работает, а гости — нет: проблема в iptables или неверной подсети.

---

## Нет интернета на основной сети после Apply

- Убедитесь, что правила iptables затрагивают **только** гостевую подсеть
- Проверьте, нет ли `uci firewall.startup_xray_guest` (удалите через uninstall или install-autostart)
- Перезагрузите роутер **без USB**, затем вставьте флешку

---

## USB не монтируется

```bash
dmesg | tail -30
ls /mnt/
block info   # если доступно на прошивке
```

Проверьте файловую систему и порт USB. Panel ждёт mount до **180 секунд** (`boot-xiaomi-vless.sh`).

---

## Сброс panel без удаления Xray

```bash
sh deploy/reset-panel.sh
```

Или полная [деинсталляция](installation.md#деинсталляция) с флагами `--keep-xray` / `--router-only`.

---

## Полезные команды

```bash
# Статус
pidof panel xray

# Логи
tail -f /data/xiaomi-vless-boot.log
tail -f /mnt/usb-*/xiaomi-vless/panel.log
tail -f /mnt/usb-*/xiaomi-vless/xray-startup.log

# Версия panel
/mnt/usb-*/xiaomi-vless/panel -version

# sysctl (rp_filter для TProxy)
sysctl net.ipv4.conf.all.rp_filter
```

Если проблема не описана здесь — см. [экспорт чата](chat-vless-xiaomi-be7000.md) с историей ручной отладки Xray.
