# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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

[0.3.0]: https://github.com/TAIIOK/xi-ray/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/TAIIOK/xi-ray/releases/tag/v0.2.0
