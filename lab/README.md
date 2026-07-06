# Lab VM (Multipass)

Эмулятор роутера для тестов panel, Xray и guest iptables без физического BE7000.

## Требования

- [Multipass](https://multipass.run/install) (macOS / Linux / Windows)
- Go 1.22+ (для сборки бинарника panel)

## Одна команда

Из корня репозитория:

```bash
make lab-up
# или
./lab/scripts/lab-up.sh
```

Скрипт:

1. Запускает VM Ubuntu 24.04 (`xiaomi-vless-lab`), если её ещё нет
2. Монтирует репозиторий в `/home/ubuntu/xiaomi-vless`
3. Собирает `panel` под CPU VM (arm64 на Apple Silicon, amd64 на Intel)
4. Устанавливает panel и скрипты в `/mnt/usb-lab/` (имитация USB-флешки)
5. Скачивает Xray и geo-файлы
6. Настраивает `br-lan` (192.168.31.0/24) и `br-guest` (192.168.33.0/24)
7. Запускает systemd-сервисы

Откройте **http://&lt;vm-ip&gt;:7777/onboarding** — логин `admin` / `admin`.

## Команды

| Команда | Описание |
|---------|----------|
| `make lab-up` | Создать или обновить lab VM |
| `make lab-down` | Остановить VM |
| `make lab-purge` | Удалить VM |
| `make lab-shell` | Shell в VM |
| `make lab-status` | Сервисы, iptables, проверка SOCKS |
| `make lab-guest-test` | Guest netns и проверка связности |
| `make lab-deploy` | Собрать и заменить только panel (VM не трогаем) |
| `make lab-deploy-full` | Полный redeploy: panel + скрипты + systemd + **чистый** `panel.json` (без серверов, onboarding заново) |
| `make lab-update-test` | E2E self-update: panel-updater apply на lab VM (без GitHub) |
| `make lab-reset-password` | Сбросить только пароль на `admin` / `admin` |

**Сохранить настройки** при full deploy: `./lab/scripts/lab-deploy.sh --full --keep-config`

После `lab-deploy-full`: логин **`admin` / `admin`**, onboarding с нуля, серверов нет.

## Обновление билда без пересоздания VM

После изменений в коде на уже поднятой lab VM:

```bash
make lab-deploy          # быстро: новый panel + restart
make lab-deploy-full     # panel + скрипты + systemd + чистый panel.json
./lab/scripts/lab-deploy.sh --full --keep-config   # full deploy, сохранить panel.json
```

Или напрямую:

```bash
./lab/scripts/lab-deploy.sh
./lab/scripts/lab-deploy.sh --full
./lab/scripts/lab-deploy.sh --full --reset
```

`LAB_SKIP_BUILD=1` — не собирать, взять уже готовый `dist/panel-linux-*`.

Типичный цикл разработки:

```bash
make lab-up              # один раз
# правки в коде…
make lab-deploy          # после каждой сборки
make lab-guest-test      # проверка guest netns (создаёт namespace)
make lab-update-test     # E2E self-update через panel-updater.sh
```

## Имитация гостевого устройства

Внутри VM (или через `make lab-guest-test`):

```bash
sudo sh /mnt/usb-lab/xiaomi-vless/guest-netns.sh
sudo ip netns exec guest-test curl -4 https://ifconfig.me
sudo iptables -t nat -L XRAY_GUEST_TCP -v -n
```

Трафик с `192.168.33.10` проходит через guest iptables — та же логика, что на роутере.

## Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `LAB_VM_NAME` | `xiaomi-vless-lab` | Имя инстанса Multipass |
| `LAB_CPUS` | `2` | Количество vCPU |
| `LAB_MEM` | `2G` | ОЗУ |
| `LAB_DISK` | `8G` | Размер диска |
| `LAB_IMAGE` | `24.04` | Образ Ubuntu |
| `LAB_RECREATE` | `0` | Установите `1`, чтобы удалить и пересоздать VM |
| `LAB_PURGE` | `0` | Установите `1` вместе с `lab-down`, чтобы удалить VM |
| `LAB_SKIP_BUILD` | `0` | Установите `1` для `lab-deploy` — не пересобирать panel |

## Что не эмулируется

- Hotplug USB и проблемы холодной загрузки Xiaomi
- Реальная пропускная способность Wi‑Fi
- Специфичные для BE7000 имена интерфейсов, кроме `br-guest` / `eth0`

Для логики panel, конфига Xray, iptables и onboarding lab-среды достаточно.

## QEMU OpenWrt (ближе к роутеру)

Для тестов **procd autostart**, **cron**, **panel-updater** как на BE7000:

```bash
make qemu-up          # OpenWrt 24.10 в QEMU
make qemu-status
make qemu-guest-test
```

Подробнее: [qemu/README.md](qemu/README.md).
