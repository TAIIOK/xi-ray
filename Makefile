.PHONY: build build-arm64 test tidy run install uninstall uninstall-yes install-autostart release-bundle tag-release deploy \
	lab-up lab-down lab-purge lab-shell lab-status lab-guest-test lab-routing-test lab-routing-e2e-test lab-reset-password lab-deploy lab-deploy-full

export GOTOOLCHAIN ?= go1.22.10

BIN=dist/panel
ARM_BIN=dist/panel-linux-arm64
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -s -w \
	-X github.com/taiiok/xiaomi-vless/internal/version.Version=$(VERSION) \
	-X github.com/taiiok/xiaomi-vless/internal/version.Commit=$(COMMIT) \
	-X github.com/taiiok/xiaomi-vless/internal/version.BuildDate=$(BUILD_DATE)

build:
	go build -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/panel

build-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(ARM_BIN) ./cmd/panel

test:
	go test ./...

tidy:
	go mod tidy

run:
	go run ./cmd/panel -config ./deploy/panel.json.example

install: build-arm64
	sh deploy/install.sh

install-autostart:
	sh deploy/install-autostart.sh

uninstall:
	sh deploy/uninstall.sh

uninstall-yes:
	sh deploy/uninstall.sh --yes

deploy:
	scp $(ARM_BIN) root@192.168.31.1:/mnt/usb-ed49605f/xiaomi-vless/panel

release-bundle: build-arm64
	VERSION=$(VERSION) sh scripts/build-release-bundle.sh

tag-release:
	@test -n "$(VERSION)" || (echo "Usage: make tag-release VERSION=v1.0.0" >&2; exit 1)
	git tag $(VERSION)
	git push origin $(VERSION)

lab-up:
	sh lab/scripts/lab-up.sh

lab-down:
	sh lab/scripts/lab-down.sh

lab-purge:
	LAB_PURGE=1 sh lab/scripts/lab-down.sh

lab-shell:
	sh lab/scripts/lab-shell.sh

lab-status:
	sh lab/scripts/lab-status.sh

lab-guest-test:
	sh lab/scripts/lab-guest-test.sh

lab-routing-test:
	sh lab/scripts/lab-routing-test.sh

lab-routing-e2e-test:
	sh lab/scripts/lab-routing-e2e-test.sh

lab-reset-password:
	sh lab/scripts/lab-reset-password.sh

lab-deploy:
	sh lab/scripts/lab-deploy.sh

lab-deploy-full:
	sh lab/scripts/lab-deploy.sh --full
