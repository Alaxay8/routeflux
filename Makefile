APP_NAME := routeflux
BUILD_DIR := bin
GO_FILES := $(shell find . -type f -name '*.go' -not -path './bin/*' -not -path './dist/*' -not -path './.cache/*')
VERSION ?= $(shell (git describe --tags --always --dirty 2>/dev/null || printf '0.0.0-dev') | sed 's/^v//')
PACKAGE_ARCH ?= mipsel_24kc
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf 'unknown')
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -s -w \
	-X github.com/Alaxay8/routeflux/internal/buildinfo.Version=$(VERSION) \
	-X github.com/Alaxay8/routeflux/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/Alaxay8/routeflux/internal/buildinfo.BuildDate=$(BUILD_DATE)

.PHONY: build test test-verbose coverage coverage-runtime lint test-integration build-openwrt build-openwrt-x86_64 package-openwrt package-release fmt

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) ./cmd/routeflux

test:
	go test ./...

test-verbose:
	go test -v ./...

coverage:
	go test -coverprofile=coverage-parser.out ./internal/parser
	go tool cover -func=coverage-parser.out
	go test -coverprofile=coverage-probe.out ./internal/probe
	go tool cover -func=coverage-probe.out
	go test -coverpkg=./internal/backend/xray -coverprofile=coverage-backend.out ./internal/backend
	go tool cover -func=coverage-backend.out
	go test -coverprofile=coverage-store.out ./internal/store
	go tool cover -func=coverage-store.out

coverage-runtime:
	./scripts/coverage-runtime.sh

lint:
	@out="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$out" ]; then \
		printf '%s\n' "$$out"; \
		exit 1; \
	fi
	go vet ./...
	go test ./...

fmt:
	go fmt ./...

build-openwrt:
	VERSION=$(VERSION) ./scripts/build-openwrt.sh

build-openwrt-x86_64:
	VERSION=$(VERSION) OUTPUT_DIR=bin/openwrt/x86_64 GOARCH=amd64 ./scripts/build-openwrt.sh

test-integration: build-openwrt-x86_64
	ROUTEFLUX_RUN_OPENWRT_INTEGRATION=1 \
	ROUTEFLUX_OPENWRT_ROUTEFLUX_BIN=$(CURDIR)/bin/openwrt/x86_64/routeflux \
	go test -count=1 -v ./test/integration/openwrt

package-openwrt: build-openwrt
	VERSION=$(VERSION) ARCH=$(PACKAGE_ARCH) ./scripts/package-openwrt.sh

package-release:
	VERSION=$(VERSION) ./scripts/package-release.sh
