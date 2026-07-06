# QEMU OpenWrt Lab

Полноценный эмулятор **OpenWrt** (armsr/armv8) для тестов panel, Xray, guest iptables и **procd autostart** — максимально близко к Xiaomi BE7000.

В отличие от Multipass lab (Ubuntu + systemd), здесь:
- **OpenWrt 24.10** в QEMU
- **procd** + `/etc/init.d/xiaomi-vless-*`
- **cron** watchdog для panel
- **USB-флешка** как отдельный virtio-диск → `/mnt/usb-lab`
- `/data` → symlink на flash

## Требования

- macOS Apple Silicon или Linux **aarch64** host
- [QEMU](https://www.qemu.org/): `brew install qemu`
- Go 1.22+ (сборка panel)
- ~500 MB для образов OpenWrt (скачаются автоматически)

## Быстрый старт

```bash
make qemu-up
# Panel: http://127.0.0.1:7777/onboarding  (admin / admin)
```

Первый запуск:
1. Скачивает OpenWrt `armsr/armv8` + U-Boot
2. Создаёт USB-диск `usb-lab.qcow2`
3. Запускает QEMU в фоне
4. Собирает `panel` для `linux/arm64`
5. Провизит гостя по SSH (opkg, Xray, procd, cron)

## Команды

| Команда | Описание |
|---------|----------|
| `make qemu-up` | Скачать образы, запустить QEMU, провизить panel |
| `make qemu-down` | Остановить QEMU |
| `make qemu-purge` | Остановить + удалить runtime (образы сохраняются) |
| `make qemu-shell` | SSH в OpenWrt (`root@127.0.0.1:2222`) |
| `make qemu-status` | procd, panel, iptables, USB mount |
| `make qemu-deploy` | Пересобрать и заменить panel (procd restart) |
| `make qemu-deploy-full` | Полный reprovision (скрипты + autostart) |
| `make qemu-guest-test` | Guest netns + connectivity |

Полный redeploy с чистым `panel.json`:

```bash
./lab/qemu/qemu-deploy.sh --full --reset
```

Сохранить конфиг:

```bash
./lab/qemu/qemu-deploy.sh --full --keep-config
```

## Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `QEMU_OWRT_VERSION` | `24.10.0` | Версия OpenWrt |
| `QEMU_MEMORY` | `1024` | RAM (MB) |
| `QEMU_CPUS` | `2` | vCPU |
| `QEMU_SSH_PORT` | `2222` | SSH port-forward на host |
| `QEMU_PANEL_PORT` | `7777` | Panel port-forward |
| `QEMU_RECREATE` | `0` | `1` — остановить и провизить заново |
| `QEMU_PURGE_IMAGES` | `0` | `1` с `qemu-purge` — удалить скачанные образы |
| `QEMU_LAN_SUBNET` | — | `192.168.31.0/24` — LAN как на BE7000 |

## Архитектура

```
Host (macOS)
  qemu-system-aarch64
    ├─ virtio disk: OpenWrt rootfs (persistent)
    ├─ virtio disk: usb-lab.qcow2 → /mnt/usb-lab
    ├─ eth0 (LAN): 192.168.1.0/24, OpenWrt 192.168.1.1 → hostfwd :7777, :2222
    └─ eth1 (WAN): 192.0.2.0/24 → DHCP 192.0.2.15, интернет (как в [доке OpenWrt](https://openwrt.org/docs/guide-user/virtualization/qemu))
```

Внутри гостя:
- `/mnt/usb-lab/xiaomi-vless/` — panel, скрипты, panel.json
- `/mnt/usb-lab/xray/` — Xray binary + geo
- `/data/` → `/mnt/usb-lab/data/` — startup_user.sh, boot hooks
- `/etc/init.d/xiaomi-vless-panel` — procd (как на роутере)

## Guest device test

```bash
make qemu-shell
# внутри OpenWrt:
sh /mnt/usb-lab/xiaomi-vless/guest-netns.sh
ip netns exec guest-test curl -4 https://ifconfig.me
iptables -t nat -L XRAY_GUEST_TCP -v -n
```

Или: `make qemu-guest-test`

## Self-update test

QEMU lab использует **procd**, не systemd — именно здесь проверять update/autostart как на роутере:

```bash
make qemu-update-test   # offline E2E: apply + rollback через panel-updater.sh
```

Или вручную через UI:

1. `make qemu-up`
2. Открыть Settings → Update (GitHub)
3. Проверить: `/etc/init.d/xiaomi-vless-panel restart`, `panel-updater.sh` без `systemctl`

Логи:
```bash
tail -f /mnt/usb-lab/xiaomi-vless/panel-update.log
logread | grep panel
```

## Multipass vs QEMU

| | Multipass | QEMU OpenWrt |
|---|---|---|
| Скорость запуска | быстрее | медленнее (образы + opkg) |
| OS | Ubuntu 24.04 | OpenWrt 24.10 |
| Autostart | systemd | **procd + cron** |
| Близость к BE7000 | средняя | **высокая** |
| Разработка UI/iptables | отлично | отлично |
| Update/regression | может пропустить procd-баги | **ловит** |

Рекомендуемый цикл:
- ежедневно: `make lab-up` / `make lab-deploy`
- перед релизом: `make qemu-up` + update test + guest test

## Что не эмулируется

- Wi‑Fi, реальные интерфейсы BE7000
- Hotplug USB (диск всегда подключён как virtio)
- Холодная загрузка с флешкой / WAN-гонки Xiaomi
- Полный fw4 (останавливается для raw iptables lab)

## Файлы

```
lab/qemu/
  qemu-up.sh              # главный entrypoint
  qemu-common.sh          # shared helpers
  download-images.sh      # OpenWrt + U-Boot
  create-usb-disk.sh      # usb-lab.qcow2
  provision-openwrt.sh    # runs inside guest
  network-setup-openwrt.sh
  panel.json
  images/                 # gitignored
  runtime/                # gitignored (pid, ssh known_hosts)
```

## Troubleshooting

**SSH не поднимается:**
```bash
tail -f lab/qemu/runtime/qemu.log
# подождать 1–2 мин после первого boot
```

**Panel не отвечает:**
```bash
make qemu-shell
/etc/init.d/xiaomi-vless-panel restart
logread | tail -30
```

**Полный сброс:**
```bash
QEMU_PURGE_IMAGES=1 make qemu-purge
make qemu-up
```
