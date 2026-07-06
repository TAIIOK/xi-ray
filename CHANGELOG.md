# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.4.0] - 2026-07-06

### Added

- **QEMU OpenWrt lab** (`lab/qemu/`): `make qemu-up`, `qemu-deploy`, `qemu-deploy-full`, `qemu-repair`, `qemu-guest-test`, `qemu-update-test` — procd/cron environment closer to BE7000 than Multipass
- Offline opkg bundle install for QEMU (host-side `.ipk` extract; fixes 0-byte stubs for `curl`, `ip`, `ca-bundle`)
- `internal/httpclient` — explicit CA bundle paths for HTTPS on OpenWrt (`/etc/ssl/certs/ca-certificates.crt`)
- `make qemu-update-test` — E2E panel apply + rollback on OpenWrt via `panel-updater.sh` (procd, no systemd)
- Guest bridge carrier via dummy interface in Multipass `lab/network-setup.sh`

### Fixed

- **`startup_xray_guest.sh`**: BusyBox ash `if ! cmd &` always entered fail-open branch — false «failed to launch xray» loop with procd respawn
- Startup script lock prevents concurrent `startup_xray_guest.sh` runs; launch check uses `pidof` only
- Guest network detection treats bridge `UNKNOWN` + IP as up (QEMU `br-guest` without Wi‑Fi port)
- Subscription fetch TLS on OpenWrt when system cert pool is empty
- `install-autostart.sh` skips `cp` when source and destination are the same (BusyBox)
- Panel procd init sets `SSL_CERT_FILE` for HTTPS client
- QEMU staging upload uses tar stream (fixes `opkg-root/etc: not a regular file` on deploy-full)
- QEMU provision restores `ca-bundle` after failed `opkg --force-reinstall` (chicken-and-egg SSL)

### Changed

- `lab/README.md`, `docs/development.md` — QEMU lab documentation

## [0.3.6] - 2026-07-06

### Fixed

- `panel-updater.sh` no longer aborts apply when `systemctl start` is slow (`set -e` + 10s timeout); waits up to 30s for HTTP readiness
- Panel startup no longer spawns competing `panel-updater resume` during `restarting` phase (race that could leave panel down)
- Rollback enabled for `dev` builds when `panel.previous` exists (same version label no longer hides rollback)

## [0.3.5] - 2026-07-06

### Added

- Integration tests for panel self-update: `scripts/test-update.sh`, `internal/server/update_handlers_test.go`, `internal/update/service_integration_test.go`
- Lab E2E update test: `make lab-update-test` (panel-updater apply on Multipass VM without GitHub)

### Fixed

- Panel self-update on systemd/lab no longer installs OpenWrt init.d hooks that break `xiaomi-vless-panel.service`
- `install-autostart.sh` skips init.d and panel cron watchdog when host uses systemd; uses `INSTALL_DIR` paths on lab instead of `/data`
- `panel-updater.sh` passes `XIAOMI_VLESS_USE_SYSTEMD` and removes legacy init.d after update hooks
- Lab provision removes incompatible init.d before `systemctl enable`
- Update resume retries `extracting` phase if archive is present; `CanDownload` disabled during active download
- Bundle `manifest.platform` validated before apply (prevents wrong-arch installs on Intel lab)

## [0.3.3] - 2026-07-04

### Fixed

- Panel apply no longer stuck on «Установка…»: stale `applying` phase auto-resumes after 90s
- Updater runs detached with output to `panel-update.log`; startup resume no longer blocks or deadlocks on flock
- Settings apply flow uses polling instead of page reload during install
- Rollback version shown with `v` prefix; rollback button hidden when `panel.previous` matches current version

## [0.3.2] - 2026-07-04

### Added

- Embedded `panel-updater.sh` auto-sync on panel startup (installs or refreshes stale script)
- Rollback target version shown in Settings (read from `panel.previous`)
- i18n strings for rollback confirm, progress, and status

### Fixed

- Panel rollback no longer kills the HTTP handler before restart (async via `nohup`)
- `panel-updater.sh` uses `systemctl` on lab/systemd hosts; cleans orphan processes before restart
- Rollback phase set to `restarting` until service is back; `rolled_back` only after successful restart
- Lab `make lab-deploy-full` installs updater from staging (Multipass mount read fix) and reliably starts panel
- Lab deploy always copies current `panel-updater.sh` alongside panel binary

## [0.3.0] - 2026-07-04

### Added

- **LICENSE** (MIT) and **CONTRIBUTING.md** for open-source contributors
- **Xray update from Settings**: download latest Xray-core + geo files from GitHub (`POST /api/xray/update`, `GET /api/xray/status`)
- **Dashboard update banner**: shows when a newer panel release is available on GitHub (links to Settings)
- **Subscriptions page redesign**: stats cards, subscription cards with node counts, relative timestamps, auto-refresh policy badge

### Changed

- Subscriptions API (`/api/subscriptions-list`) now includes `node_count` per subscription and `total_nodes`
- Settings panel update section has anchor `#panel-update` for deep links from dashboard

### Fixed

- Panel update check no longer hangs indefinitely (30s timeout, IPv4-first dial to GitHub API)
- Update banner and settings only offer releases **newer** than the running version (semver; ignores `-dirty` suffix)
- Dashboard no longer triggers GitHub check on every page load

### Included from pre-release work (since v0.2.0)

- i18n (ru/en) across UI
- Fail-open mode for guest Wi‑Fi when VPN fails
- Routing editor with presets and preview
- Lab VM (Multipass) for development without physical router
- Watchdog with Telegram notifications
- Guest network detection and validation

## [0.2.0]

### Added

- In-panel self-update with resume download, atomic apply, and rollback
- GitHub Actions CI and release pipeline
- Embedded version via `-version` flag
- Release `install.sh` and `scripts/quick-install.sh` for one-command setup

[0.3.6]: https://github.com/TAIIOK/xi-ray/compare/v0.3.5...v0.3.6
[0.3.5]: https://github.com/TAIIOK/xi-ray/compare/v0.3.3...v0.3.5
[0.3.3]: https://github.com/TAIIOK/xi-ray/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/TAIIOK/xi-ray/compare/v0.3.0...v0.3.2
[0.3.0]: https://github.com/TAIIOK/xi-ray/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/TAIIOK/xi-ray/releases/tag/v0.2.0
